package spa

// ═══════════════════════════════════════════════════════════════════════════════
//  SPA Application — HTTP Handlers
// ═══════════════════════════════════════════════════════════════════════════════
//
//  This is the CORE of the Go backend. Every API endpoint that the single-page
//  application calls is defined here as a method on the App struct.
//
//  HOW IT WORKS:
//  The frontend sends all data as FormData (url-encoded key/value pairs).
//  Handlers call r.FormValue() to read fields, run validation, query the
//  database using parameterized SQL, and return JSON responses.
//
//  SECURITY PRINCIPLES:
//  1. NEVER trust the client. All input is validated server-side.
//  2. All SQL uses ? placeholders — parameterized queries prevent injection.
//  3. Protected routes call requireAuthJSON() first. If the session cookie
//     is missing/invalid, we return 401 immediately.
//  4. Input lengths are bounded to prevent resource exhaustion.
//  5. Only expected fields are read from the request — no mass assignment.
//  6. Passwords are hashed with bcrypt, never stored in plain text.
//  7. JSON responses use the Go encoder which safely escapes strings.

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
// These limit input sizes to match the database schema and prevent abuse.
// The frontend enforces the same limits, but we ALWAYS check server-side too.

const (
	maxPostContentLength       = 2000 // Max characters in a post body
	maxCommentContentLength    = 500  // Max characters in a comment
	maxMessageContentLength    = 1000 // Max characters in a private message
	maxPostTitleLength         = 60   // Max characters in a post title
	maxCategoriesPerPost       = 10   // Max categories a post can belong to
)

// ─── App ─────────────────────────────────────────────────────────────────────

// App groups all SPA HTTP handlers and the WebSocket hub.
// We create ONE App at startup and register its methods as route handlers.
type App struct {
	Hub *Hub
}

// NewApp creates a new App with a fresh WebSocket hub.
// Call this once in main().
func NewApp() *App {
	return &App{Hub: NewHub()}
}

// ─── Shell: Serve the HTML Page ─────────────────────────────────────────────

// ShellHandler serves index.html — the SPA entry point.
// Every page load hits this endpoint. The browser then loads app.js, which
// bootstraps the entire application (checks session, renders UI, etc.).
func (app *App) ShellHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/static/index.html")
}

// ─── Authentication ─────────────────────────────────────────────────────────

// MeHandler returns the currently logged-in user's profile.
// - If the session cookie is valid → return user object (200).
// - If no session or invalid session → return 401.
// The frontend uses this on every page load to check if the user is logged in.
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

// RegisterHandler creates a new user account.
//
// Expected form fields: nickname, age, gender, first_name, last_name, email, password.
// SECURITY:
// - All fields are validated server-side.
// - Email and nickname are checked for duplicates before insertion.
// - Password is hashed with bcrypt (cost 14) before storage.
// - On success, a session cookie is set so the user is immediately logged in.
func (app *App) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	// ── Parse & validate all form fields ────────────────────────────────
	// The frontend sends these as FormData. We read them as raw strings
	// and validate BEFORE touching the database.
	nickname := strings.TrimSpace(r.FormValue("nickname"))
	ageValue := strings.TrimSpace(r.FormValue("age"))
	gender := strings.ToLower(strings.TrimSpace(r.FormValue("gender")))
	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")

	// Convert age to integer and validate all fields
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

	// ── Check for duplicate email or nickname ────────────────────────────
	// We check BEFORE inserting to avoid constraint violations. The check
	// uses parameterized SQL (safe from injection).
	if userExistsByLogin(email, nickname) {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "email or nickname already exists"})
		return
	}

	// ── Hash password & insert into database ────────────────────────────
	// bcrypt adds a random salt and runs 2^14 iterations of key derivation.
	// This makes password cracking extremely expensive even if the database
	// is compromised. We never store plaintext passwords.
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

	// ── Create session & set HttpOnly cookie ────────────────────────────
	// The session is stored in the database and referenced by a random UUID.
	// The cookie is HttpOnly (JS can't read it) + SameSite=Lax (CSRF protection).
	sessionID, err := auth.CreateSession(int(userID64))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to create session"})
		return
	}
	auth.SetSessionCookie(w, sessionID)

	// ── Respond with the new user profile ───────────────────────────────
	user, _ := loadUserProfile(int(userID64))
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "user": user})
}

// LoginHandler authenticates a user by email or nickname.
//
// Expected form fields: login (email or nickname), password.
// SECURITY:
// - The login field is matched against both email AND nickname via a
//   parameterized query (safe from SQL injection).
// - Password comparison uses bcrypt.CompareHashAndPassword (constant-time).
// - The error message is intentionally vague ("invalid credentials") so an
//   attacker can't tell whether the login or the password was wrong.
// - On success, a session cookie is set (HttpOnly + SameSite=Lax).
func (app *App) LoginHandler(w http.ResponseWriter, r *http.Request) {
	login := strings.ToLower(strings.TrimSpace(r.FormValue("login")))
	password := r.FormValue("password")

	if login == "" || password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "missing login or password"})
		return
	}

	// ── Look up user by email or nickname ───────────────────────────────
	// The WHERE clause checks both fields using OR. Both values are bound
	// as parameters (?) so SQL injection is not possible here.
	var userID int
	var hashedPassword string
	err := database.DB.QueryRow(`
		SELECT id, password
		FROM users
		WHERE LOWER(email) = ? OR LOWER(COALESCE(nickname, username)) = ?
		LIMIT 1
	`, login, login).Scan(&userID, &hashedPassword)
	if err == sql.ErrNoRows {
		// Deliberately vague error — don't reveal whether login exists
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "invalid credentials"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "database error"})
		return
	}

	// ── Verify password against bcrypt hash ─────────────────────────────
	// bcrypt.CompareHashAndPassword is designed to be timing-attack resistant.
	if !auth.CheckPasswordHash(password, hashedPassword) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "invalid credentials"})
		return
	}

	// ── Create session ──────────────────────────────────────────────────
	// The session links this browser to the user ID via a random UUID cookie.
	sessionID, err := auth.CreateSession(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to create session"})
		return
	}
	auth.SetSessionCookie(w, sessionID)

	user, _ := loadUserProfile(userID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": user})
}

// LogoutHandler clears the session: deletes the session row from the DB and
// sets an expired cookie in the browser (which removes it).
// Safe to call even when no session exists — we just clear the cookie.
func (app *App) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Delete the session from the database if it exists
	cookie, err := r.Cookie(auth.SESSION_COOKIE_NAME)
	if err == nil {
		_ = auth.DeleteSession(cookie.Value)
	}

	// Clear the cookie by setting an expired date
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SESSION_COOKIE_NAME,
		Value:    "",
		Expires:  time.Unix(0, 0), // Unix epoch = already expired
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ─── Categories ─────────────────────────────────────────────────────────────

// CategoriesHandler returns all post categories.
// No authentication required — categories are public.
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

// PostsHandler dispatches to either listPosts (GET) or createPost (POST)
// depending on the HTTP method. This is how we handle multiple methods on
// the same path without an external router.
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

// PostHandler returns a single post by ID with its full comment thread.
// Path parameter: {post_id} (from the URL).
func (app *App) PostHandler(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.Atoi(r.PathValue("post_id"))
	if err != nil || postID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid post id"})
		return
	}
	app.getPost(w, r, postID)
}

// listPosts returns the post feed, optionally filtered by category_id.
// No authentication required — anyone can browse posts.
func (app *App) listPosts(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserFromRequest(r) // 0 if not logged in (fine)
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

// createPost creates a new forum post. Requires authentication.
//
// Expected form fields: title, content, categories[].
// SECURITY:
// - Requires valid session (via requireAuthJSON).
// - Title length limited to 60 chars, content to 2000 chars.
// - Category IDs must be valid positive integers.
// - Categories are limited to maxCategoriesPerPost.
// - Uses a database transaction so categories + post are created atomically.
func (app *App) createPost(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireAuthJSON(w, r)
	if !ok {
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	content := strings.TrimSpace(r.FormValue("content"))
	categoryValues := r.Form["categories"]

	// Validate input lengths and presence
	if title == "" || len(title) > maxPostTitleLength ||
		content == "" || len(content) > maxPostContentLength ||
		len(categoryValues) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid post"})
		return
	}

	// Validate each category ID is a positive integer
	// Limit the number of categories to prevent abuse
	if len(categoryValues) > maxCategoriesPerPost {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "too many categories"})
		return
	}

	categoryIDs := make([]int, 0, len(categoryValues))
	for _, v := range categoryValues {
		id, err := strconv.Atoi(v)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid category"})
			return
		}
		categoryIDs = append(categoryIDs, id)
	}

	// ── Transaction: insert post + category associations atomically ─────
	// Using a transaction ensures that if any step fails, all changes are
	// rolled back. We never end up with an orphaned post or missing links.
	tx, err := database.DB.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "database error"})
		return
	}
	defer tx.Rollback() // no-op if Commit() was already called

	result, err := tx.Exec(
		`INSERT INTO posts (user_id, title, content) VALUES (?, ?, ?)`,
		userID, title, content,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "failed to create post"})
		return
	}

	postID, _ := result.LastInsertId()

	// Link each category to the new post
	for _, catID := range categoryIDs {
		if _, err := tx.Exec(
			`INSERT INTO post_categories (post_id, category_id) VALUES (?, ?)`,
			postID, catID,
		); err != nil {
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
// No authentication required, but if a user is logged in we include
// their reaction state (liked/disliked).
func (app *App) getPost(w http.ResponseWriter, r *http.Request, postID int) {
	userID, _ := auth.GetUserFromRequest(r) // 0 if not logged in

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
	`, userID, postID).Scan(
		&post.ID, &post.UserID, &post.Nickname,
		&post.Title, &post.Content, &post.CreatedAt,
		&post.ReactedTo, &post.Likes, &post.Dislikes,
	)
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

// CommentsHandler creates a new comment on a post. Requires authentication.
// The post_id comes from the URL path parameter {post_id}.
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
	// Validate length server-side (matches the frontend limit)
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
// or comment. Requires authentication.
//
// Behaviour:
//   - No existing reaction → insert it.
//   - Same reaction exists → remove it (unlike / undislike toggle).
//   - Opposite reaction exists → switch to the new type.
//
// SECURITY:
// - Requires valid session.
// - post_id or comment_id must be a positive integer (validated).
// - type must be exactly 1 or -1 (validated).
// - Target (post/comment) must exist in the database (validated).
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

	// Validate that exactly one target ID is provided, and it's a valid int
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

	// Verify the target post or comment actually exists
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

	// Check if the user already has a reaction on this target
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
		// Remove the existing reaction first
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
		// If same type was clicked again → toggle off (already removed above)
		if existingType == reactionType {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "action": "removed"})
			return
		}
	}

	// Insert (or re-insert with the new reaction type)
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

// ─── Private Messages ────────────────────────────────────────────────────────

// ChatContactsHandler returns all users except the current one, ordered by
// most recent message then alphabetically. Requires authentication.
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

// ChatMessagesHandler returns the conversation between the current user and
// another user (specified by ?with=userID). Supports pagination via
// ?before_id=lastMessageID.
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

// MessageSendHandler stores a new private message and forwards it to both
// participants via WebSocket. Requires authentication.
//
// SECURITY:
// - senderID is always from the session (never from the form).
// - receiverID must be a positive integer (validated).
// - Content length is capped at maxMessageContentLength.
// - Self-messaging is blocked (senderID != receiverID).
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

	// Push the new message to both sender and receiver via WebSocket
	// This gives the sender immediate feedback and alerts the receiver
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
// The user is authenticated via their session cookie during the upgrade.
func (app *App) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	if app.Hub == nil {
		app.Hub = NewHub()
	}
	app.Hub.Serve(w, r)
}

// ─── Internal Helpers ────────────────────────────────────────────────────────

// requireAuthJSON checks for a valid session. If the user is not authenticated,
// it writes a 401 JSON response and returns false. All protected routes
// should call this first.
//
// SECURITY: This is the GATE that protects all authenticated endpoints.
// The user ID is always read from the session cookie, never from the request
// body, so an attacker can't impersonate another user by modifying form fields.
func (app *App) requireAuthJSON(w http.ResponseWriter, r *http.Request) (int, bool) {
	userID, err := auth.GetUserFromRequest(r)
	if err != nil || userID == 0 {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "unauthorized"})
		return 0, false
	}
	return userID, true
}

// loadContacts returns all users except the current one, ordered by most
// recent message time (descending), then alphabetically by nickname.
// Online status is determined from the WebSocket Hub (live connection).
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
		if err := rows.Scan(
			&c.ID, &c.Nickname, &c.FirstName, &c.LastName, &lastMsg,
		); err != nil {
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

// loadMessages returns the conversation between two users, ordered oldest-first.
// Supports "load more" pagination via beforeID (load messages older than this ID).
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
		if err := rows.Scan(
			&m.ID, &m.SenderID, &m.ReceiverID,
			&m.SenderName, &m.ReceiverName,
			&m.Content, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		m.CreatedAt = forum.FormatDate(m.CreatedAt)
		messages = append(messages, m)
	}

	// Reverse so the UI gets oldest-first order (the query returned newest-first)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// insertMessage saves a new private message to the database and returns it.
// The message is timestamped with the current server time.
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

// displayName returns the nickname of a user, or "Unknown" if not found.
func (app *App) displayName(userID int) string {
	user, err := loadUserProfile(userID)
	if err != nil {
		return "Unknown"
	}
	return user.Nickname
}

// ─── Standalone helpers ──────────────────────────────────────────────────────

// loadUserProfile fetches a full user profile from the database by user ID.
func loadUserProfile(userID int) (*APIUser, error) {
	var u APIUser
	err := database.DB.QueryRow(`
		SELECT id, email, COALESCE(nickname, username),
			COALESCE(age, 0), COALESCE(gender, ''),
			COALESCE(first_name, ''), COALESCE(last_name, '')
		FROM users WHERE id = ?
	`, userID).Scan(
		&u.ID, &u.Email, &u.Nickname,
		&u.Age, &u.Gender, &u.FirstName, &u.LastName,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// userExistsByLogin checks whether an email or nickname is already registered.
// Used during registration to prevent duplicate accounts.
func userExistsByLogin(email, nickname string) bool {
	var exists int
	_ = database.DB.QueryRow(`
		SELECT 1 FROM users
		WHERE LOWER(email) = ? OR LOWER(COALESCE(nickname, username)) = ?
		LIMIT 1
	`, email, strings.ToLower(nickname)).Scan(&exists)
	return exists == 1
}

// loadPostCategories fetches all category names for a given post.
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

// convertComments transforms forum.Comment structs into APIComment structs
// suitable for JSON serialization.
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

// writeJSON sets the Content-Type header and encodes the payload as JSON.
// This is the only function that writes responses — all handlers use it.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// nullInt converts 0 to nil (SQL NULL) and non-zero to the value.
// Used for optional post_id/comment_id in the reactions table (one must
// be NULL to distinguish post reactions from comment reactions).
func nullInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

// nowString returns the current time formatted as RFC 3339.
func nowString() string {
	return time.Now().Format(time.RFC3339)
}

// ─── Validation Functions ────────────────────────────────────────────────────
// These validate user input BEFORE it reaches the database.
// Each function checks one field and returns true if valid.
// The frontend has matching validation — but we NEVER trust the client.

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
