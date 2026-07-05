package spa

// ═══════════════════════════════════════════════════════════════════════════
//  SPA Application — HTTP Handlers
// ═══════════════════════════════════════════════════════════════════════════
//
// This is the heart of the Go backend.  Every API endpoint used by the
// single‑page application is defined here.  The App struct ties together
// the database, session management, and the WebSocket hub.
//
// ─── Architecture ─────────────────────────────────────────────────────────
// The frontend sends all data as application/x-www-form-urlencoded (the
// default for FormData in JavaScript).  Handlers parse r.FormValue(), run
// basic validation, query the database, and return JSON.
//
// Protected routes call requireAuthJSON() first – if the session cookie is
// missing or invalid the handler returns 401 immediately.
//
// ─── Routes ───────────────────────────────────────────────────────────────
// See main.go for the full route table.

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"forum/internals/auth"
	"forum/internals/database"
	"forum/internals/forum"
)

// ─── Constants ──────────────────────────────────────────────────────────────

const (
	maxPostContentLength    = 2000
	maxCommentContentLength = 500
	maxMessageContentLength = 1000
)

// ─── App ─────────────────────────────────────────────────────────────────────

// App groups the SPA handlers and the WebSocket hub.
type App struct {
	Hub *Hub
}

// NewApp creates a new App and starts the hub.
func NewApp() *App {
	return &App{Hub: NewHub()}
}

// ─── Shell: Serve the Single HTML Page ───────────────────────────────────────

// ShellHandler serves the single index.html that bootstraps the entire SPA.
// Every page load hits this endpoint; the JavaScript router (app.js) takes
// over from there.
func (app *App) ShellHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/static/index.html")
}

// ─── Authentication ──────────────────────────────────────────────────────────

// MeHandler returns the currently logged‑in user's profile.
// If no valid session exists it returns 401 – the frontend shows the auth view.
func (app *App) MeHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserFromRequest(r)
	if err != nil || userID == 0 {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "unauthorized"})
		return
	}

	user, err := loadUserProfile(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to load user"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": user})
}

// RegisterHandler creates a new account.  Expected form fields:
// nickname, age, gender, first_name, last_name, email, password.
func (app *App) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	// ── Parse & validate ──────────────────────────────────────────────
	nickname := strings.TrimSpace(r.FormValue("nickname"))
	ageValue := strings.TrimSpace(r.FormValue("age"))
	gender := strings.ToLower(strings.TrimSpace(r.FormValue("gender")))
	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")

	age, err := strconv.Atoi(ageValue)
	if err != nil ||
		!isValidNickname(nickname) ||
		!isValidAge(age) ||
		!isValidGender(gender) ||
		!isValidFullName(firstName) ||
		!isValidFullName(lastName) ||
		!isValidEmail(email) ||
		!isValidPassword(password) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid registration data"})
		return
	}

	// ── Check duplicates ──────────────────────────────────────────────
	if userExistsByLogin(email, nickname) {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "email or nickname already exists"})
		return
	}

	// ── Hash password & insert ────────────────────────────────────────
	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to hash password"})
		return
	}

	result, err := database.DB.Exec(`
		INSERT INTO users (email, username, nickname, age, gender, first_name, last_name, password)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, email, nickname, nickname, age, gender, firstName, lastName, hashedPassword)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to create user"})
		return
	}

	userID64, _ := result.LastInsertId()

	// ── Create session & set cookie ───────────────────────────────────
	sessionID, err := auth.CreateSession(int(userID64))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to create session"})
		return
	}
	auth.SetSessionCookie(w, sessionID)

	// ── Respond ───────────────────────────────────────────────────────
	user, _ := loadUserProfile(int(userID64))
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "user": user})
}

// LoginHandler authenticates a user.  The `login` field accepts either
// a nickname or an email (the server checks both).
func (app *App) LoginHandler(w http.ResponseWriter, r *http.Request) {
	login := strings.ToLower(strings.TrimSpace(r.FormValue("login")))
	password := r.FormValue("password")

	if login == "" || password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "missing login or password"})
		return
	}

	// ── Look up user by email or nickname ─────────────────────────────
	var userID int
	var hashedPassword string
	err := database.DB.QueryRow(`
		SELECT id, password
		FROM users
		WHERE LOWER(email) = ? OR LOWER(COALESCE(nickname, username)) = ?
		LIMIT 1
	`, login, login).Scan(&userID, &hashedPassword)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "invalid credentials"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "database error"})
		return
	}

	// ── Verify password ───────────────────────────────────────────────
	if !auth.CheckPasswordHash(password, hashedPassword) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "invalid credentials"})
		return
	}

	// ── Create session ────────────────────────────────────────────────
	sessionID, err := auth.CreateSession(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to create session"})
		return
	}
	auth.SetSessionCookie(w, sessionID)

	user, _ := loadUserProfile(userID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": user})
}

// LogoutHandler clears the session cookie and deletes the session row.
// Safe to call even when no session exists.
func (app *App) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SESSION_COOKIE_NAME)
	if err == nil {
		_ = auth.DeleteSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     auth.SESSION_COOKIE_NAME,
		Value:    "",
		Expires:  time.Unix(0, 0),
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ─── Categories ──────────────────────────────────────────────────────────────

// CategoriesHandler returns the full list of available post categories.
func (app *App) CategoriesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`SELECT id, name FROM categories ORDER BY name ASC`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to load categories"})
		return
	}
	defer rows.Close()

	categories := make([]APICategory, 0)
	for rows.Next() {
		var cat APICategory
		if err := rows.Scan(&cat.ID, &cat.Name); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to read categories"})
			return
		}
		categories = append(categories, cat)
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "categories": categories})
}

// ─── Posts ───────────────────────────────────────────────────────────────────

// PostsHandler dispatches to listPosts (GET) or createPost (POST).
func (app *App) PostsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		app.listPosts(w, r)
	case http.MethodPost:
		app.createPost(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "method not allowed"})
	}
}

// PostHandler returns a single post with its full comment thread.
func (app *App) PostHandler(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.Atoi(r.PathValue("post_id"))
	if err != nil || postID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid post id"})
		return
	}
	app.getPost(w, r, postID)
}

// listPosts returns the post feed, optionally filtered by category_id.
func (app *App) listPosts(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserFromRequest(r)
	categoryID, _ := strconv.Atoi(r.URL.Query().Get("category_id"))

	posts, err := forum.GetPosts(categoryID, 0, userID, 0, 0, 0, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to load posts"})
		return
	}

	feed := make([]APIPost, 0, len(posts))
	for _, p := range posts {
		feed = append(feed, APIPost{
			ID:           p.PostID,
			UserID:       p.UserID,
			Nickname:     p.Username,
			Title:        p.Title,
			Content:      p.Content,
			CreatedAt:    p.CreatedAt,
			Categories:   p.Categories,
			Likes:        p.Likes,
			Dislikes:     p.Dislikes,
			ReactedTo:    p.ReactedTo,
			CommentCount: p.CommentsLen,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "posts": feed})
}

// createPost inserts a new post and its category associations in a transaction.
func (app *App) createPost(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireAuthJSON(w, r)
	if !ok {
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	content := strings.TrimSpace(r.FormValue("content"))
	categoryValues := r.Form["categories"]

	if title == "" || len(title) > 60 ||
		content == "" || len(content) > maxPostContentLength ||
		len(categoryValues) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid post"})
		return
	}

	// Validate category IDs
	categoryIDs := make([]int, 0, len(categoryValues))
	for _, v := range categoryValues {
		id, err := strconv.Atoi(v)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid category"})
			return
		}
		categoryIDs = append(categoryIDs, id)
	}

	// ── Transaction ───────────────────────────────────────────────────
	tx, err := database.DB.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "database error"})
		return
	}
	defer tx.Rollback()

	result, err := tx.Exec(`INSERT INTO posts (user_id, title, content) VALUES (?, ?, ?)`, userID, title, content)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to create post"})
		return
	}

	postID, _ := result.LastInsertId()

	for _, catID := range categoryIDs {
		if _, err := tx.Exec(`INSERT INTO post_categories (post_id, category_id) VALUES (?, ?)`, postID, catID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to attach categories"})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to save post"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "post_id": postID})
}

// getPost loads a single post with all its comments and categories.
func (app *App) getPost(w http.ResponseWriter, r *http.Request, postID int) {
	userID, _ := auth.GetUserFromRequest(r)

	var post APIPost
	err := database.DB.QueryRow(`
		SELECT p.id, p.user_id, COALESCE(u.nickname, u.username),
			p.title, p.content, p.created_at,
			COALESCE(re.type, 0) AS reacted_to,
			(SELECT COUNT(*) FROM reactions WHERE post_id = p.id AND type = 1) AS likes,
			(SELECT COUNT(*) FROM reactions WHERE post_id = p.id AND type = -1) AS dislikes
		FROM posts p
		JOIN users u ON u.id = p.user_id
		LEFT JOIN reactions re ON p.id = re.post_id AND re.user_id = ?
		WHERE p.id = ?
	`, userID, postID).Scan(&post.ID, &post.UserID, &post.Nickname, &post.Title, &post.Content, &post.CreatedAt, &post.ReactedTo, &post.Likes, &post.Dislikes)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "post not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to load post"})
		return
	}

	post.CreatedAt = forum.FormatDate(post.CreatedAt)
	post.Categories = loadPostCategories(postID)

	comments, err := forum.GetCommentsByPost(userID, postID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to load comments"})
		return
	}
	post.Comments = convertComments(comments)
	post.CommentCount = len(post.Comments)

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "post": post})
}

// ─── Comments ────────────────────────────────────────────────────────────────

// CommentsHandler creates a new comment on a post (POST only).
func (app *App) CommentsHandler(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.Atoi(r.PathValue("post_id"))
	if err != nil || postID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid post id"})
		return
	}
	app.createComment(w, r, postID)
}

func (app *App) createComment(w http.ResponseWriter, r *http.Request, postID int) {
	userID, ok := app.requireAuthJSON(w, r)
	if !ok {
		return
	}

	content := strings.TrimSpace(r.FormValue("content"))
	if content == "" || len(content) > maxCommentContentLength {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid comment"})
		return
	}

	result, err := database.DB.Exec(
		`INSERT INTO comments (post_id, user_id, content) VALUES (?, ?, ?)`,
		postID, userID, content,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to create comment"})
		return
	}

	commentID, _ := result.LastInsertId()
	comment := APIComment{
		ID:        int(commentID),
		PostID:    postID,
		UserID:    userID,
		Nickname:  app.displayName(userID),
		Content:   content,
		CreatedAt: forum.FormatDate(nowString()),
	}

	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "comment": comment})
}

// ─── Reactions (Likes / Dislikes) ────────────────────────────────────────────

// ReactionHandler toggles a like (type=1) or dislike (type=-1) on a post
// or comment.  Accepts either post_id or comment_id (not both).
//
// Behaviour:
//   - If no reaction exists → insert it.
//   - If the same reaction exists → remove it (unlike / undislike).
//   - If the opposite reaction exists → switch (change like to dislike, etc.).
func (app *App) ReactionHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireAuthJSON(w, r)
	if !ok {
		return
	}

	postIDStr := strings.TrimSpace(r.FormValue("post_id"))
	commentIDStr := strings.TrimSpace(r.FormValue("comment_id"))
	typeStr := strings.TrimSpace(r.FormValue("type"))

	reactionType, err := strconv.Atoi(typeStr)
	if (postIDStr == "" && commentIDStr == "") ||
		(postIDStr != "" && commentIDStr != "") ||
		(reactionType != 1 && reactionType != -1) || err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid parameters"})
		return
	}

	postID := 0
	commentID := 0
	if postIDStr != "" {
		postID, err = strconv.Atoi(postIDStr)
		if err != nil || postID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid post id"})
			return
		}
	} else {
		commentID, err = strconv.Atoi(commentIDStr)
		if err != nil || commentID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid comment id"})
			return
		}
	}

	// Verify target exists
	var exists bool
	if postID > 0 {
		_ = database.DB.QueryRow(`SELECT EXISTS (SELECT 1 FROM posts WHERE id = ?)`, postID).Scan(&exists)
	} else {
		_ = database.DB.QueryRow(`SELECT EXISTS (SELECT 1 FROM comments WHERE id = ?)`, commentID).Scan(&exists)
	}
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "target not found"})
		return
	}

	// Check existing reaction
	var existingType int
	var found bool
	if postID > 0 {
		err = database.DB.QueryRow(
			`SELECT type FROM reactions WHERE user_id = ? AND post_id = ? AND comment_id IS NULL`,
			userID, postID,
		).Scan(&existingType)
		found = err == nil
	} else {
		err = database.DB.QueryRow(
			`SELECT type FROM reactions WHERE user_id = ? AND comment_id = ? AND post_id IS NULL`,
			userID, commentID,
		).Scan(&existingType)
		found = err == nil
	}

	if found {
		// Remove the existing reaction (regardless of type)
		if postID > 0 {
			_, _ = database.DB.Exec(
				`DELETE FROM reactions WHERE user_id = ? AND post_id = ? AND comment_id IS NULL`,
				userID, postID,
			)
		} else {
			_, _ = database.DB.Exec(
				`DELETE FROM reactions WHERE user_id = ? AND comment_id = ? AND post_id IS NULL`,
				userID, commentID,
			)
		}
		// If the same type was clicked again → just remove (toggle off)
		if existingType == reactionType {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "action": "removed"})
			return
		}
	}

	// Insert (or re-insert with new type)
	_, err = database.DB.Exec(
		`INSERT INTO reactions (user_id, post_id, comment_id, type) VALUES (?, ?, ?, ?)`,
		userID, nullInt(postID), nullInt(commentID), reactionType,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to save reaction"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "action": "set"})
}

// nullInt converts 0 to nil (SQL NULL) and non-zero to the value.
// This is needed because the reactions table expects NULL for the
// unused post_id/comment_id column.
func nullInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

// ─── Private Messages ────────────────────────────────────────────────────────

// ChatContactsHandler returns the user list for the chat sidebar, ordered
// by last message time (most recent first) then alphabetically.
func (app *App) ChatContactsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireAuthJSON(w, r)
	if !ok {
		return
	}

	contacts, err := app.loadContacts(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to load contacts"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "contacts": contacts})
}

// ChatMessagesHandler returns messages between the current user and another
// user.  Supports pagination via `before_id` (the last message ID seen).
// Returns up to 10 messages per call.
func (app *App) ChatMessagesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireAuthJSON(w, r)
	if !ok {
		return
	}

	otherUserID, err := strconv.Atoi(r.URL.Query().Get("with"))
	if err != nil || otherUserID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid contact id"})
		return
	}

	beforeID, _ := strconv.Atoi(r.URL.Query().Get("before_id"))

	messages, err := app.loadMessages(userID, otherUserID, beforeID, 10)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to load messages"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "messages": messages})
}

// MessageSendHandler stores a new message and forwards it over WebSocket.
func (app *App) MessageSendHandler(w http.ResponseWriter, r *http.Request) {
	senderID, ok := app.requireAuthJSON(w, r)
	if !ok {
		return
	}

	receiverID, err := strconv.Atoi(strings.TrimSpace(r.FormValue("receiver_id")))
	content := strings.TrimSpace(r.FormValue("content"))

	if err != nil || receiverID <= 0 || content == "" || len(content) > maxMessageContentLength {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid message"})
		return
	}
	if senderID == receiverID {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "you cannot message yourself"})
		return
	}

	message, err := app.insertMessage(senderID, receiverID, content)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to save message"})
		return
	}

	// Push the new message to both participants via WebSocket
	payload := map[string]any{
		"type":    "message",
		"message": message,
		"contact": receiverID,
		"sender":  senderID,
	}
	app.Hub.SendToUser(senderID, payload)
	app.Hub.SendToUser(receiverID, payload)

	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "message": message})
}

// WebSocketHandler upgrades the HTTP request to a WebSocket connection.
func (app *App) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	if app.Hub == nil {
		app.Hub = NewHub()
	}
	app.Hub.Serve(w, r)
}

// ─── Internal Helpers ────────────────────────────────────────────────────────

// loadContacts loads all users except the current one, ordered by most recent
// message then alphabetically.  Online status is determined from the Hub.
func (app *App) loadContacts(userID int) ([]ChatContact, error) {
	rows, err := database.DB.Query(`
		SELECT
			u.id,
			COALESCE(u.nickname, u.username),
			COALESCE(u.first_name, ''),
			COALESCE(u.last_name, ''),
			(
				SELECT MAX(pm.created_at)
				FROM private_messages pm
				WHERE (pm.sender_id = u.id AND pm.receiver_id = ?)
				   OR (pm.sender_id = ? AND pm.receiver_id = u.id)
			) AS last_message_at
		FROM users u
		WHERE u.id != ?
		ORDER BY
			CASE WHEN last_message_at IS NULL THEN 1 ELSE 0 END,
			last_message_at DESC,
			LOWER(COALESCE(u.nickname, u.username)) ASC
	`, userID, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contacts := make([]ChatContact, 0)
	for rows.Next() {
		var c ChatContact
		var lastMsg sql.NullString
		if err := rows.Scan(&c.ID, &c.Nickname, &c.FirstName, &c.LastName, &lastMsg); err != nil {
			return nil, err
		}
		if lastMsg.Valid {
			c.LastMessageAt = lastMsg.String
		}
		c.Online = app.Hub.IsOnline(c.ID)
		contacts = append(contacts, c)
	}

	return contacts, nil
}

// loadMessages returns a conversation between two users, newest first,
// ordered oldest‑to‑newest for the UI.  Supports "load more" pagination.
func (app *App) loadMessages(userID, otherUserID, beforeID, limit int) ([]ChatMessage, error) {
	query := `
		SELECT pm.id, pm.sender_id, pm.receiver_id,
			COALESCE(s.nickname, s.username),
			COALESCE(r.nickname, r.username),
			pm.content, pm.created_at
		FROM private_messages pm
		JOIN users s ON s.id = pm.sender_id
		JOIN users r ON r.id = pm.receiver_id
		WHERE ((pm.sender_id = ? AND pm.receiver_id = ?) OR (pm.sender_id = ? AND pm.receiver_id = ?))
	`
	args := []any{userID, otherUserID, otherUserID, userID}
	if beforeID > 0 {
		query += " AND pm.id < ?"
		args = append(args, beforeID)
	}
	query += " ORDER BY pm.id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]ChatMessage, 0)
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.ID, &m.SenderID, &m.ReceiverID, &m.SenderName, &m.ReceiverName, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.CreatedAt = forum.FormatDate(m.CreatedAt)
		messages = append(messages, m)
	}

	// Reverse so the UI gets oldest‑first order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// insertMessage saves a new private message to the database and returns it.
func (app *App) insertMessage(senderID, receiverID int, content string) (ChatMessage, error) {
	result, err := database.DB.Exec(
		`INSERT INTO private_messages (sender_id, receiver_id, content) VALUES (?, ?, ?)`,
		senderID, receiverID, content,
	)
	if err != nil {
		return ChatMessage{}, err
	}

	messageID, _ := result.LastInsertId()
	message := ChatMessage{
		ID:           int(messageID),
		SenderID:     senderID,
		ReceiverID:   receiverID,
		SenderName:   app.displayName(senderID),
		ReceiverName: app.displayName(receiverID),
		Content:      content,
		CreatedAt:    forum.FormatDate(nowString()),
	}

	return message, nil
}

// requireAuthJSON is a guard that checks for a valid session.  If the user is
// not authenticated it writes a 401 JSON response and returns false.
func (app *App) requireAuthJSON(w http.ResponseWriter, r *http.Request) (int, bool) {
	userID, err := auth.GetUserFromRequest(r)
	if err != nil || userID == 0 {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "unauthorized"})
		return 0, false
	}
	return userID, true
}

// displayName returns the nickname of a user, or "Unknown" if not found.
func (app *App) displayName(userID int) string {
	user, err := loadUserProfile(userID)
	if err != nil {
		return "Unknown"
	}
	return user.Nickname
}

// ─── Standalone helpers ──────────────────────────────────────────────────────

// loadUserProfile fetches a full user profile from the database.
func loadUserProfile(userID int) (*APIUser, error) {
	var u APIUser
	err := database.DB.QueryRow(`
		SELECT id, email, COALESCE(nickname, username),
			COALESCE(age, 0), COALESCE(gender, ''),
			COALESCE(first_name, ''), COALESCE(last_name, '')
		FROM users WHERE id = ?
	`, userID).Scan(&u.ID, &u.Email, &u.Nickname, &u.Age, &u.Gender, &u.FirstName, &u.LastName)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// userExistsByLogin checks whether an email or nickname is already taken.
func userExistsByLogin(email, nickname string) bool {
	var exists int
	_ = database.DB.QueryRow(`
		SELECT 1 FROM users
		WHERE LOWER(email) = ? OR LOWER(COALESCE(nickname, username)) = ?
		LIMIT 1
	`, email, strings.ToLower(nickname)).Scan(&exists)
	return exists == 1
}

// loadPostCategories fetches category names for a given post.
func loadPostCategories(postID int) []string {
	rows, err := database.DB.Query(`
		SELECT c.name FROM categories c
		JOIN post_categories pc ON pc.category_id = c.id
		WHERE pc.post_id = ?
		ORDER BY c.name ASC
	`, postID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	categories := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			categories = append(categories, name)
		}
	}
	return categories
}

// convertComments transforms forum.Comment slices into APIComment slices.
func convertComments(comments []forum.Comment) []APIComment {
	result := make([]APIComment, 0, len(comments))
	for _, c := range comments {
		result = append(result, APIComment{
			ID:        c.CommentID,
			PostID:    c.PostID,
			UserID:    c.UserID,
			Nickname:  c.Username,
			Content:   c.Content,
			CreatedAt: c.CreatedAt,
		})
	}
	return result
}

// writeJSON is a tiny helper that sets the Content-Type header and encodes
// the payload as JSON.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// ─── Validation ──────────────────────────────────────────────────────────────

func isValidNickname(value string) bool {
	if len(value) < 3 || len(value) > 24 {
		return false
	}
	for i, r := range value {
		if i == 0 && !unicode.IsLetter(r) {
			return false
		}
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func isValidAge(age int) bool {
	return age >= 13 && age <= 120
}

func isValidGender(value string) bool {
	switch value {
	case "male", "female", "other", "prefer_not_to_say":
		return true
	default:
		return false
	}
}

func isValidFullName(value string) bool {
	if len(value) < 1 || len(value) > 60 {
		return false
	}
	for _, r := range value {
		if !(unicode.IsLetter(r) || r == ' ' || r == '-' || r == '\'') {
			return false
		}
	}
	return true
}

func isValidEmail(value string) bool {
	return strings.Contains(value, "@") && strings.Contains(value, ".")
}

func isValidPassword(password string) bool {
	if len(password) < 8 || len(password) > 72 {
		return false
	}
	var hasUpper, hasLower, hasNumber, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasNumber = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}
	return hasUpper && hasLower && hasNumber && hasSymbol
}

func nowString() string {
	return time.Now().Format(time.RFC3339)
}
