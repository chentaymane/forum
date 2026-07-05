// ═════════════════════════════════════════════════════════════════════════════
//  FORUM — Single Page Application (Client)
// ═════════════════════════════════════════════════════════════════════════════
//
//  HOW IT WORKS:
//  This is a single-page app (SPA). The server sends one HTML page, and then
//  ALL rendering happens here in JavaScript. When the user clicks things, we
//  call the Go backend via fetch() (REST API), get JSON back, and update the
//  DOM ourselves. No page reloads.
//
//  SECURITY:
//  - All user text is escaped with escapeHtml() before being inserted into
//    the DOM. This prevents XSS (cross-site scripting) attacks.
//  - Session cookies are HttpOnly (JS can't steal them) + SameSite=Lax
//    (prevents CSRF). The server handles all real validation — we never
//    trust the client.
//  - The api() helper sends credentials with every request so the server
//    can authenticate via the session cookie.
//
//  STATE MANAGEMENT:
//  The global `state` object holds all app data (user, posts, contacts…).
//  When state changes, we call the matching render*() function to update
//  the DOM. No virtual DOM, no framework — just direct DOM manipulation.
//
//  FILE STRUCTURE:
//    1. State & DOM references
//    2. Utility functions (escapeHtml, debounce, api…)
//    3. Render functions (update the DOM from state)
//    4. Data loading (fetch from the Go API)
//    5. Chat popup logic
//    6. Authentication (login / register / logout)
//    7. WebSocket connection (real‑time messaging)
//    8. Event binding (wires UI events once at startup)
//    9. Bootstrap (entry point)
// ═════════════════════════════════════════════════════════════════════════════

// ─── Global State ───────────────────────────────────────────────────────────
// This object holds ALL application data. Every render*() function reads from
// here and writes to the DOM. We never read the DOM for data — state is the
// single source of truth.

const state = {
  user: null,              // The logged-in user object (null = not logged in)
  categories: [],          // Available post categories (loaded from /api/categories)
  posts: [],               // Current feed of posts (loaded from /api/posts)
  activePost: null,        // Post currently shown in the detail modal
  contacts: [],            // All users except the current one (for chat)
  activeContact: null,     // Currently selected chat partner (null = no conversation open)
  messages: [],            // Messages in the active conversation
  seenMessageIds: new Set(), // IDs of messages already rendered (prevents dupes from WebSocket)
  oldestMessageId: null,   // ID of the oldest loaded message (for "load more" pagination)
  hasMoreMessages: true,   // Whether there are older messages to load
  loadingMessages: false,  // Prevents concurrent loadMore() calls
  socket: null,            // WebSocket connection (null = disconnected)
  contactSearch: "",       // Text in the sidebar's "People" search
  popupSearch: "",         // Text in the chat popup's search field
  popupTab: "recent",      // Active tab in chat popup: "recent" | "all"
  popupOpen: false,        // Whether the chat popup is currently visible
};

// ─── DOM References (cached once) ──────────────────────────────────────────
// Instead of calling document.getElementById() a hundred times, we grab every
// element ONCE here and reuse them through the `el` object. This is slightly
// faster and makes the code cleaner.

const $ = (id) => document.getElementById(id);

const el = {
  // Auth view (shown when logged out)
  authView:         $("auth-view"),
  appView:          $("app-view"),
  showRegister:     $("show-register"),
  showLogin:        $("show-login"),
  registerCard:     $("register-card"),
  logoutButton:     $("logout-button"),
  userBadge:        $("user-badge"),
  connectionStatus: $("connection-status"),
  toast:            $("toast"),

  // Auth forms
  loginForm:        $("login-form"),
  registerForm:     $("register-form"),

  // Post composer & feed
  postForm:         $("post-form"),
  postsList:        $("posts-list"),
  categoryFilter:   $("category-filter"),
  categoryList:     $("category-list"),
  refreshPosts:     $("refresh-posts"),

  // Post detail modal
  postModal:        $("post-modal"),
  detailTitle:      $("detail-title"),
  detailMeta:       $("detail-meta"),
  detailCategories: $("detail-categories"),
  detailBody:       $("detail-content"),
  detailReactions:  $("detail-reactions"),
  commentsList:     $("comments-list"),
  commentForm:      $("comment-form"),
  closeDetail:      $("close-detail"),

  // People sidebar (left column)
  contactsList:     $("contacts-list"),
  contactSearch:    $("contact-search"),

  // Chat popup (floating messenger overlay)
  chatButton:       $("chat-button"),
  chatPopup:        $("chat-popup"),
  chatPopupClose:   $("chat-popup-close"),
  chatTabRecent:    $("chat-tab-recent"),
  chatTabAll:       $("chat-tab-all"),
  chatPopupSearch:  $("chat-popup-search"),
  chatPopupList:    $("chat-popup-list"),
  chatConversation: $("chat-conversation"),
  chatConvBack:     $("chat-conv-back"),
  chatConvUser:     $("chat-conv-user"),
  chatConvMessages: $("chat-conv-messages"),
  messageForm:      $("message-form"),
};

// ═════════════════════════════════════════════════════════════════════════════
//  UTILITY FUNCTIONS
// ═════════════════════════════════════════════════════════════════════════════

// --- showToast ---------------------------------------------------------------
// Shows a notification in the bottom-right corner. Auto-hides after 2.5s.
// Pass isError=true to show a red error toast.
// SECURITY: Uses textContent (not innerHTML) so no XSS risk.

function showToast(message, isError = false) {
  el.toast.textContent = message;
  el.toast.classList.remove("hidden", "toast-error");
  if (isError) el.toast.classList.add("toast-error");
  clearTimeout(showToast._timer);
  showToast._timer = setTimeout(() => el.toast.classList.add("hidden"), 2500);
}

// --- setAuthenticated ---------------------------------------------------------
// Toggles between the auth view (login/register) and the app view (feed/chat).
// Called after login, register, logout, or when loadMe() finds no session.

function setAuthenticated(yes) {
  el.authView.classList.toggle("hidden", yes);
  el.appView.classList.toggle("hidden", !yes);
  el.logoutButton.classList.toggle("hidden", !yes);
  el.userBadge.classList.toggle("hidden", !yes);
  el.chatButton.classList.toggle("hidden", !yes);
}

// --- api ----------------------------------------------------------------------
// Wrapper around fetch() that ALL API calls go through.
//
// WHY: We always need to send cookies (credentials: "same-origin") so the
// server can authenticate us via the session cookie. We also always parse
// JSON and throw on HTTP errors so the calling code can use try/catch.
//
// SECURITY: credentials: "same-origin" ensures cookies are only sent to our
// own server, never to third parties. The server then validates the session
// cookie to determine who we are.

async function api(path, options = {}) {
  const response = await fetch(path, {
    credentials: "same-origin",
    ...options,
  });

  const ct = response.headers.get("content-type") || "";
  const body = ct.includes("application/json")
    ? await response.json()
    : await response.text();

  if (!response.ok) {
    const msg = typeof body === "string" ? body : body.error || response.statusText;
    throw new Error(msg);
  }

  return body;
}

// --- escapeHtml ---------------------------------------------------------------
// Converts special HTML characters to their entity equivalents.
//
// WHY: If we insert user-provided text into the DOM using innerHTML, an
// attacker could inject <script> tags or other malicious HTML. This function
// prevents that by converting <, >, &, ", ' to safe alternatives.
//
// SECURITY: Call this EVERY TIME you put user data inside innerHTML. If
// you use textContent instead, escaping is not needed (textContent is always
// safe). We use a mix of both approaches in this codebase.

function escapeHtml(value) {
  const map = {
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  };
  return String(value).replace(/[&<>"']/g, (ch) => map[ch]);
}

// --- debounce & throttle ------------------------------------------------------
// These limit how often a function can run. Used for search inputs (debounce)
// and scroll events (throttle) so we don't do expensive work on every keystroke.

function debounce(fn, ms = 250) {
  let timer;
  return (...args) => {
    clearTimeout(timer);
    timer = setTimeout(() => fn(...args), ms);
  };
}

function throttle(fn, ms = 250) {
  let last = 0, timer = null;
  return (...args) => {
    const now = Date.now();
    const remaining = ms - (now - last);
    if (remaining <= 0) {
      last = now;
      fn(...args);
      return;
    }
    if (timer) return;
    timer = setTimeout(() => {
      last = Date.now();
      timer = null;
      fn(...args);
    }, remaining);
  };
}

// ═════════════════════════════════════════════════════════════════════════════
//  RENDER FUNCTIONS
// ═════════════════════════════════════════════════════════════════════════════
//
//  Each render*() function reads from the global `state` and updates one part
//  of the DOM. They are called whenever the relevant state changes.
//
//  PATTERN: state → render*() → DOM. Never the other way around.

// ─── Helper: reactionButtons ─────────────────────────────────────────────────
// Returns HTML for the like/dislike button pair.
// SECURITY: post.id, post.likes, post.dislikes are integers (safe for
// innerHTML). We only use escapeHtml() for strings.

function reactionButtons(post) {
  const liked = post.reacted_to === 1 ? "active" : "";
  const disliked = post.reacted_to === -1 ? "active" : "";
  const pid = post.id;
  return `
    <span class="reaction-group">
      <button type="button" class="btn-reaction btn-like ${liked}" data-post-id="${pid}" data-type="1">👍 ${post.likes || 0}</button>
      <button type="button" class="btn-reaction btn-dislike ${disliked}" data-post-id="${pid}" data-type="-1">👎 ${post.dislikes || 0}</button>
    </span>
  `;
}

// --- renderCategories ---------------------------------------------------------
// Populates the category filter dropdown AND the checkbox chips in the
// new-post form. Called once at startup and whenever categories change.

function renderCategories() {
  const filterOpts = [`<option value="">All categories</option>`];
  const chips = [];

  for (const cat of state.categories) {
    // SECURITY: cat.id is an integer (safe), cat.name is escaped
    filterOpts.push(`<option value="${cat.id}">${escapeHtml(cat.name)}</option>`);
    chips.push(`
      <label class="chip chip-check">
        <input type="checkbox" name="categories" value="${cat.id}" />
        <span>${escapeHtml(cat.name)}</span>
      </label>
    `);
  }

  el.categoryFilter.innerHTML = filterOpts.join("");
  el.categoryList.innerHTML = chips.join("");
}

// --- renderPosts ---------------------------------------------------------------
// Renders the post feed from state.posts. Each post becomes a clickable card.
// When clicked, the card's data-post-id is used to fetch the full post detail.

function renderPosts() {
  if (!state.posts.length) {
    el.postsList.innerHTML = `<p class="empty-state">No posts yet. Be the first to write one!</p>`;
    return;
  }

  el.postsList.innerHTML = state.posts
    .map((post) => {
      // SECURITY: All user text is escaped. post.id is an integer (safe).
      return `
        <article class="card post-card" data-post-id="${post.id}">
          <div class="card-topline">
            <span>${escapeHtml(post.nickname)}</span>
            <span>${escapeHtml(post.created_at)}</span>
          </div>
          <h3>${escapeHtml(post.title)}</h3>
          <p class="card-text">${escapeHtml(post.content)}</p>
          <div class="chip-row">${formatCategories(post.categories || [])}</div>
          <div class="card-footer">
            <span class="reaction-group">
              <button type="button" class="btn-reaction btn-like ${post.reacted_to === 1 ? "active" : ""}" data-post-id="${post.id}" data-type="1">👍 ${post.likes || 0}</button>
              <button type="button" class="btn-reaction btn-dislike ${post.reacted_to === -1 ? "active" : ""}" data-post-id="${post.id}" data-type="-1">👎 ${post.dislikes || 0}</button>
            </span>
            <span class="comment-count">${post.comment_count || 0} comments</span>
          </div>
        </article>
      `;
    })
    .join("");
}

// --- renderContacts -----------------------------------------------------------
// Renders the "People" sidebar (left column). Shows all users with online/
// offline indicators. Filtered by state.contactSearch text.

function renderContacts() {
  const filter = state.contactSearch.trim().toLowerCase();
  const contacts = !filter
    ? state.contacts
    : state.contacts.filter(
        (c) =>
          c.nickname.toLowerCase().includes(filter) ||
          (c.first_name || "").toLowerCase().includes(filter) ||
          (c.last_name || "").toLowerCase().includes(filter)
      );

  if (!contacts.length) {
    el.contactsList.innerHTML = `<p class="empty-state">No users found</p>`;
    return;
  }

  // SECURITY: All user-supplied text is escaped with escapeHtml()
  el.contactsList.innerHTML = contacts
    .map((c) => {
      const active = state.activeContact && state.activeContact.id === c.id;
      const fullName = [c.first_name, c.last_name].filter(Boolean).join(" ");
      const heading = fullName
        ? `${escapeHtml(c.nickname)} — ${escapeHtml(fullName)}`
        : escapeHtml(c.nickname);
      return `
        <button type="button" class="contact-item ${active ? "active" : ""}" data-contact-id="${c.id}">
          <span>
            <strong>${heading}</strong>
            <small>${c.last_message_at ? escapeHtml(c.last_message_at) : "No messages yet"}</small>
          </span>
          <span class="status-dot ${c.online ? "online" : "offline"}">${c.online ? "Online" : "Offline"}</span>
        </button>
      `;
    })
    .join("");
}

// --- renderPostDetail ---------------------------------------------------------
// Opens the post detail MODAL and fills it with the post's content, categories,
// reactions, and comments. Called after the user clicks a post card.
//
// SECURITY: Uses textContent for title, meta, and body (always safe). Uses
// escapeHtml() for all user text in innerHTML.

function renderPostDetail(post) {
  state.activePost = post;
  el.postModal.classList.remove("hidden");

  // textContent is safe — no XSS possible here
  el.detailTitle.textContent = post.title;
  el.detailMeta.textContent = `${post.nickname} · ${post.created_at}`;
  el.detailBody.textContent = post.content;

  // innerHTML with escaped content
  el.detailCategories.innerHTML = formatCategories(post.categories || []);
  el.detailReactions.innerHTML = reactionButtons(post);
  el.commentsList.innerHTML =
    (post.comments || [])
      .map(
        (c) => `
        <article class="comment-card">
          <div class="card-topline">
            <span>${escapeHtml(c.nickname)}</span>
            <span>${escapeHtml(c.created_at)}</span>
          </div>
          <p>${escapeHtml(c.content)}</p>
        </article>
      `
      )
      .join("") || `<p class="empty-state">No comments yet.</p>`;
}

// --- hidePostDetail -----------------------------------------------------------
// Closes the post detail modal. Resets the comment form for next time.

function hidePostDetail() {
  state.activePost = null;
  el.postModal.classList.add("hidden");
  el.commentForm.reset();
}

// --- renderConversation -------------------------------------------------------
// Renders messages in the chat popup. Called when messages change or when
// switching conversations. Auto-scrolls to the bottom.

function renderConversation() {
  const list = el.chatConvMessages;
  // SECURITY: All message content is escaped with escapeHtml()
  list.innerHTML =
    state.messages
      .map((msg) => {
        const mine = state.user && msg.sender_id === state.user.id;
        return `
        <article class="message-card ${mine ? "mine" : "theirs"}" data-message-id="${msg.id}">
          <div class="message-meta">
            <strong>${escapeHtml(msg.sender_name)}</strong>
            <span>${escapeHtml(msg.created_at)}</span>
          </div>
          <p>${escapeHtml(msg.content)}</p>
        </article>
      `;
      })
      .join("") || `<p class="empty-state">No messages yet. Say hello!</p>`;
  list.scrollTop = list.scrollHeight;
}

// --- upsertMessage ------------------------------------------------------------
// Adds or prepends a message to the local state, avoiding duplicates by
// checking seenMessageIds. Returns true if the message was new.

function upsertMessage(message, prepend = false) {
  if (state.seenMessageIds.has(message.id)) return false;
  state.seenMessageIds.add(message.id);
  if (prepend) state.messages.unshift(message);
  else state.messages.push(message);
  return true;
}

// ─── Helper: formatCategories ────────────────────────────────────────────────
// Converts an array of category name strings into chip HTML.
// SECURITY: Uses escapeHtml() to prevent XSS.

function formatCategories(categories) {
  return categories
    .map((name) => `<span class="chip">${escapeHtml(name)}</span>`)
    .join("");
}

// ─── Helper: userItem ────────────────────────────────────────────────────────
// Returns HTML for a user entry used in BOTH the left sidebar and the popup.
// SECURITY: All user text is escaped.

function userItem(c, active) {
  const fullName = [c.first_name, c.last_name].filter(Boolean).join(" ");
  const heading = fullName
    ? `${escapeHtml(c.nickname)} — ${escapeHtml(fullName)}`
    : escapeHtml(c.nickname);
  return `
    <button type="button" class="contact-item ${active ? "active" : ""}" data-contact-id="${c.id}">
      <span>
        <strong>${heading}</strong>
        <small>${c.last_message_at ? escapeHtml(c.last_message_at) : "No messages yet"}</small>
      </span>
      <span class="status-dot ${c.online ? "online" : "offline"}">${c.online ? "Online" : "Offline"}</span>
    </button>
  `;
}

// --- renderPopupList ----------------------------------------------------------
// Renders the chat popup's contact list based on the active tab and search text.
// Tab "recent" shows only contacts with prior messages (sorted by newest).
// Tab "all" shows every contact (sorted alphabetically).

function renderPopupList() {
  const filter = state.popupSearch.trim().toLowerCase();
  let contacts = state.contacts;

  // Filter by tab
  if (state.popupTab === "recent") {
    contacts = contacts.filter((c) => c.last_message_at);
  }

  // Filter by search text
  if (filter) {
    contacts = contacts.filter(
      (c) =>
        c.nickname.toLowerCase().includes(filter) ||
        (c.first_name || "").toLowerCase().includes(filter) ||
        (c.last_name || "").toLowerCase().includes(filter)
    );
  }

  // Sort: recent tab by last_message_at DESC, all tab alphabetically
  if (state.popupTab === "recent") {
    contacts = [...contacts].sort((a, b) => {
      if (!a.last_message_at) return 1;
      if (!b.last_message_at) return -1;
      return b.last_message_at.localeCompare(a.last_message_at);
    });
  } else {
    contacts = [...contacts].sort((a, b) =>
      a.nickname.localeCompare(b.nickname)
    );
  }

  el.chatPopupList.innerHTML = !contacts.length
    ? `<p class="empty-state">No users found</p>`
    : contacts.map((c) => userItem(c, false)).join("");
}

// --- handleReaction -----------------------------------------------------------
// Sends a like/dislike toggle request to the server and refreshes the UI.
// The "type" (1 = like, -1 = dislike) comes from the button's data-type attr.

async function handleReaction(btn) {
  const postId = btn.dataset.postId;
  const type = parseInt(btn.dataset.type, 10);
  if (!postId || !type) return;

  btn.disabled = true;
  try {
    const data = await api("/api/reactions", {
      method: "POST",
      body: new URLSearchParams({ post_id: postId, type }),
    });
    if (!data.ok) throw new Error(data.error || "reaction failed");
    // If viewing the reacted post, reload its detail; otherwise refresh the feed
    if (state.activePost && String(state.activePost.id) === postId) {
      await loadPost(postId);
    } else {
      await loadPosts();
    }
  } catch (err) {
    showToast(err.message, true);
  } finally {
    btn.disabled = false;
  }
}

// ═════════════════════════════════════════════════════════════════════════════
//  CHAT POPUP LOGIC
// ═════════════════════════════════════════════════════════════════════════════

// --- openPopup ----------------------------------------------------------------
// Opens the floating chat popup. Shows the contact list (not a conversation).

function openPopup() {
  state.popupOpen = true;
  el.chatPopup.classList.remove("hidden");
  el.chatButton.classList.add("hidden");
  // Reset to list view (hide any open conversation)
  el.chatConversation.classList.add("hidden");
  el.chatPopupList.classList.remove("hidden");
  if (state.activeContact) {
    state.activeContact = null;
    state.messages = [];
  }
  renderPopupList();
}

// --- closePopup ---------------------------------------------------------------
// Closes the chat popup and shows the floating button again.

function closePopup() {
  state.popupOpen = false;
  el.chatPopup.classList.add("hidden");
  el.chatButton.classList.remove("hidden");
  state.activeContact = null;
  state.messages = [];
}

// --- goBackToList -------------------------------------------------------------
// Goes from a chat conversation back to the contact list inside the popup.
// Also clears the search filter.

function goBackToList() {
  state.activeContact = null;
  state.messages = [];
  state.popupSearch = "";
  el.chatPopupSearch.value = "";
  el.chatConversation.classList.add("hidden");
  el.chatPopupList.classList.remove("hidden");
  renderPopupList();
}

// --- openChat -----------------------------------------------------------------
// Opens a conversation with the given contact inside the chat popup.
// Loads the last 10 messages from the server.

async function openChat(contact) {
  state.activeContact = contact;
  state.messages = [];
  state.seenMessageIds = new Set();
  state.oldestMessageId = null;
  state.hasMoreMessages = true;
  // Show conversation view, hide contact list
  el.chatConversation.classList.remove("hidden");
  el.chatPopupList.classList.add("hidden");
  el.chatConvUser.textContent = `${contact.nickname} · ${contact.online ? "🟢 Online" : "⚪ Offline"}`;
  el.messageForm.receiver_id.value = contact.id;
  renderConversation();
  await loadConversation();
  await loadContacts(); // refresh presence
}

// ═════════════════════════════════════════════════════════════════════════════
//  DATA LOADING (API calls to the Go backend)
// ═════════════════════════════════════════════════════════════════════════════
//
//  Every load*() function calls the Go API, updates state, and calls the
//  matching render*() function. Errors are caught and shown as toasts.
//
//  Race protection: loadPosts() and loadContacts() use a request sequence
//  counter (_loadPostsSeq / _loadContactsSeq) to discard stale responses.
//  This prevents old responses from overwriting newer data when the user
//  changes filters rapidly.

// --- loadMe -------------------------------------------------------------------
// Calls GET /api/me to check if the user has a valid session cookie.
// Called on EVERY page load during bootstrap().
// - If the session is valid → sets state.user, shows the app view.
// - If the session is missing/expired → sets state.user = null, shows auth view.

async function loadMe() {
  try {
    const data = await api("/api/me");
    state.user = data.user;
    setAuthenticated(true);
    el.userBadge.textContent = state.user.nickname;
    el.logoutButton.textContent = `Logout`;
    connectSocket();
  } catch {
    state.user = null;
    setAuthenticated(false);
  }
}

// --- loadCategories -----------------------------------------------------------
// Fetches post categories from the server. These rarely change, so we load
// them once at startup and cache them in state.categories.

async function loadCategories() {
  const data = await api("/api/categories");
  state.categories = data.categories || [];
  renderCategories();
}

// --- loadPosts ----------------------------------------------------------------
// Fetches the post feed, optionally filtered by category_id.
// Uses _loadPostsSeq to avoid race conditions from rapid filter changes.

let _loadPostsSeq = 0;
async function loadPosts() {
  const seq = ++_loadPostsSeq;
  const catId = el.categoryFilter.value;
  const suffix = catId ? `?category_id=${encodeURIComponent(catId)}` : "";
  const data = await api(`/api/posts${suffix}`);
  if (seq !== _loadPostsSeq) return; // discard stale response
  state.posts = data.posts || [];
  renderPosts();
}

// --- loadContacts -------------------------------------------------------------
// Fetches the list of all users (except current user) for the sidebar and
// chat popup. Also updates the popup list if it's open.

let _loadContactsSeq = 0;
async function loadContacts() {
  if (!state.user) return;
  const seq = ++_loadContactsSeq;
  const data = await api("/api/chat/contacts");
  if (seq !== _loadContactsSeq) return; // discard stale response
  state.contacts = data.contacts || [];
  renderContacts();
  if (state.popupOpen && !state.activeContact) renderPopupList();
}

// --- loadPost -----------------------------------------------------------------
// Fetches a single post with its full comment thread. Shows the result in
// the post detail modal.

async function loadPost(postId) {
  const data = await api(`/api/posts/${postId}`);
  renderPostDetail(data.post);
}

// --- loadConversation ---------------------------------------------------------
// Loads messages for the active chat conversation. Supports pagination via
// the before_id query param (load older messages).

async function loadConversation() {
  if (!state.activeContact || state.loadingMessages || !state.hasMoreMessages) return;
  state.loadingMessages = true;

  try {
    const before = state.oldestMessageId ? `&before_id=${state.oldestMessageId}` : "";
    const data = await api(`/api/chat/messages?with=${state.activeContact.id}${before}`);
    const msgs = data.messages || [];

    if (msgs.length < 10) state.hasMoreMessages = false;
    msgs.forEach((m) => upsertMessage(m, true));
    // Track the oldest message ID for pagination
    if (msgs.length) {
      state.oldestMessageId = msgs[0].id;
    }
    renderConversation();
  } finally {
    state.loadingMessages = false;
  }
}

// ═════════════════════════════════════════════════════════════════════════════
//  AUTHENTICATION
// ═════════════════════════════════════════════════════════════════════════════
//
//  The server manages sessions via HttpOnly cookies. When the user logs in
//  or registers, the server sets a "forum_session" cookie. On every
//  subsequent request, the browser sends the cookie back automatically, and
//  the server looks it up to identify the user.
//
//  SECURITY GAT:
//  1. The cookie is HttpOnly → JavaScript can't read or steal it.
//  2. SameSite=Lax prevents CSRF (other sites can't make authenticated
//     requests on behalf of the user).
//  3. The server validates every protected request via requireAuthJSON().
//  4. On the client side, we never trust what the server hasn't verified.

// --- submitLogin --------------------------------------------------------------
// Sends login credentials to POST /api/login. The server sets a session
// cookie and returns the user profile. On success, we update the UI.

async function submitLogin(form) {
  const data = await api("/api/login", {
    method: "POST",
    body: new FormData(form),
  });
  state.user = data.user;
  setAuthenticated(true);
  el.userBadge.textContent = state.user.nickname;
  form.reset();
  connectSocket();
  await afterAuthSuccess();
}

// --- submitRegister -----------------------------------------------------------
// Sends registration data to POST /api/register. The server creates the
// account, sets a session cookie, and returns the user profile.

async function submitRegister(form) {
  const data = await api("/api/register", {
    method: "POST",
    body: new FormData(form),
  });
  state.user = data.user;
  setAuthenticated(true);
  el.userBadge.textContent = state.user.nickname;
  form.reset();
  connectSocket();
  await afterAuthSuccess();
}

// --- logout -------------------------------------------------------------------
// Calls POST /api/logout to destroy the session. The server clears the
// cookie and we reset the UI to the auth view.

async function logout() {
  await api("/api/logout", { method: "POST" });
  disconnectSocket();
  state.user = null;
  state.activeContact = null;
  state.activePost = null;
  state.messages = [];
  state.seenMessageIds = new Set();
  closePopup();
  setAuthenticated(false);
  showToast("Logged out successfully");
}

// --- afterAuthSuccess ---------------------------------------------------------
// Called after login or register. Loads initial data (posts + contacts)
// and starts a 30-second interval to refresh contacts for presence updates.

async function afterAuthSuccess() {
  await Promise.all([loadPosts(), loadContacts()]);
  hidePostDetail();
  if (state.contacts.length) renderContacts();

  // Poll contacts every 30s so we see new users and presence changes
  if (window._contactsRefreshTimer) clearInterval(window._contactsRefreshTimer);
  window._contactsRefreshTimer = setInterval(async () => {
    if (state.user) try { await loadContacts(); } catch {}
  }, 30000);
}

// ═════════════════════════════════════════════════════════════════════════════
//  WEBSOCKET (real-time messaging & presence)
// ═════════════════════════════════════════════════════════════════════════════
//
//  The WebSocket connection is used for two things:
//  1. RECEIVING new private messages in real time (no polling needed).
//  2. RECEIVING presence updates (who's online/offline).
//
//  IMPORTANT: The server NEVER reads client-sent WebSocket messages. All
//  data flows server → client. The client only sends to the WebSocket to
//  trigger the server's ping/pong keep-alive. This prevents any client-side
//  injection through the WebSocket channel.

// --- connectSocket ------------------------------------------------------------
// Opens a WebSocket connection to the server. The server authenticates the
// connection using the session cookie (sent automatically because the
// WebSocket URL is on the same origin).

function connectSocket() {
  if (!state.user) return;
  disconnectSocket();

  const protocol = location.protocol === "https:" ? "wss" : "ws";
  state.socket = new WebSocket(`${protocol}://${location.host}/ws`);

  el.connectionStatus.textContent = "Connecting…";
  el.connectionStatus.className = "status-pill pending";

  state.socket.onopen = () => {
    el.connectionStatus.textContent = "🟢 Online";
    el.connectionStatus.className = "status-pill online";
  };

  state.socket.onclose = () => {
    el.connectionStatus.textContent = "🔴 Offline";
    el.connectionStatus.className = "status-pill";
  };

  state.socket.onerror = () => {
    el.connectionStatus.textContent = "🔴 Offline";
    el.connectionStatus.className = "status-pill";
  };

  // --- onmessage: handle incoming WebSocket events ---------------------------
  // The server sends JSON events. We recognize two types:
  //   - "presence": who's online (list of user IDs)
  //   - "message": a new private message was sent

  state.socket.onmessage = async (event) => {
    const payload = JSON.parse(event.data);

    // Presence update — someone came online or went offline
    if (payload.type === "presence") {
      const onlineSet = new Set(payload.online_user_ids || []);
      state.contacts = state.contacts.map((c) => ({
        ...c,
        online: onlineSet.has(c.id),
      }));
      renderContacts();
      if (state.activeContact) {
        const updated = state.contacts.find(
          (c) => c.id === state.activeContact.id
        );
        if (updated) state.activeContact = updated;
        el.chatConvUser.textContent = `${state.activeContact.nickname} · ${state.activeContact.online ? "🟢 Online" : "⚪ Offline"}`;
        renderConversation();
      }
      return;
    }

    // New private message from another user
    if (payload.type === "message" && payload.message) {
      const msg = payload.message;
      const activeId = state.activeContact ? state.activeContact.id : null;
      const involved =
        state.user &&
        (msg.sender_id === state.user.id || msg.receiver_id === state.user.id);

      if (involved) {
        // If no chat is open, just refresh contacts to show latest message
        if (!state.activeContact) {
          await loadContacts();
          return;
        }

        const matchesChat =
          msg.sender_id === activeId || msg.receiver_id === activeId;
        if (matchesChat) {
          if (upsertMessage(msg, false)) {
            state.oldestMessageId = state.messages.length
              ? state.messages[0].id
              : null;
            renderConversation();
          }
        }
        await loadContacts();
      }
    }
  };
}

// --- disconnectSocket ---------------------------------------------------------
// Closes the WebSocket connection cleanly.

function disconnectSocket() {
  if (state.socket) {
    state.socket.close();
    state.socket = null;
  }
}

// ─── Composer: sendPost ─────────────────────────────────────────────────────
// Creates a new post. The form data (title, content, categories) is sent
// to POST /api/posts. The server validates and stores it.

async function sendPost(form) {
  await api("/api/posts", {
    method: "POST",
    body: new FormData(form),
  });
  form.reset();
  await loadPosts();
}

// ─── Composer: sendComment ──────────────────────────────────────────────────
// Adds a comment to the currently viewed post. The form data is sent to
// POST /api/posts/{post_id}/comments.

async function sendComment(form) {
  if (!state.activePost) return;
  await api(`/api/posts/${state.activePost.id}/comments`, {
    method: "POST",
    body: new FormData(form),
  });
  form.reset();
  await loadPost(state.activePost.id);
}

// ─── Composer: sendMessage (chat) ───────────────────────────────────────────
// Sends a private message to the current chat contact. The form data is
// sent to POST /api/messages.

async function sendMessage(form) {
  await api("/api/messages", {
    method: "POST",
    body: new FormData(form),
  });
  form.reset();
  // The message will arrive via WebSocket, so we don't need to manually
  // add it to the conversation — the onmessage handler takes care of it.
}

// ═════════════════════════════════════════════════════════════════════════════
//  EVENT BINDING
// ═════════════════════════════════════════════════════════════════════════════
//
//  All event listeners are registered ONCE in bindEvents(), called at startup.
//  We never add/remove listeners dynamically — the DOM elements are always
//  present (they're just shown/hidden via CSS classes).
//
//  WHY: This is simpler and more reliable than adding/removing listeners
//  every time a view is shown. It also avoids memory leaks from orphaned
//  listeners.

function bindEvents() {
  // ── Auth form toggles (switch between login / register cards) ───────────
  const loginCard = el.showRegister.closest(".auth-card");
  const registerCard = el.registerCard;

  el.showRegister.addEventListener("click", (e) => {
    e.preventDefault();
    loginCard.style.display = "none";
    registerCard.style.display = "block";
  });

  el.showLogin.addEventListener("click", (e) => {
    e.preventDefault();
    registerCard.style.display = "none";
    loginCard.style.display = "block";
  });

  // ── Login form submit ───────────────────────────────────────────────────
  el.loginForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      await submitLogin(e.currentTarget);
      showToast("Welcome back!");
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Register form submit ────────────────────────────────────────────────
  el.registerForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      await submitRegister(e.currentTarget);
      showToast("Account created!");
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Logout button ───────────────────────────────────────────────────────
  el.logoutButton.addEventListener("click", async () => {
    try {
      await logout();
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Create post ─────────────────────────────────────────────────────────
  el.postForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      await sendPost(e.currentTarget);
      showToast("Post published!");
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Refresh posts button ────────────────────────────────────────────────
  el.refreshPosts.addEventListener("click", async () => {
    try {
      el.categoryFilter.value = "";
      await loadPosts();
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Category filter dropdown ────────────────────────────────────────────
  el.categoryFilter.addEventListener("change", async () => {
    try {
      await loadPosts();
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Click post card to view detail ──────────────────────────────────────
  // Uses event delegation: one listener on the container, checks what was
  // clicked. We check for reaction buttons FIRST (stopPropagation prevents
  // the card click from also firing).
  el.postsList.addEventListener("click", async (e) => {
    const btn = e.target.closest(".btn-reaction");
    if (btn) {
      e.stopPropagation();
      await handleReaction(btn);
      return;
    }
    const card = e.target.closest("[data-post-id]");
    if (!card) return;
    try {
      await loadPost(card.dataset.postId);
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Close post detail modal (X button or backdrop click) ───────────────
  el.closeDetail.addEventListener("click", hidePostDetail);
  el.postModal.addEventListener("click", (e) => {
    // Only close if the backdrop itself was clicked (not the modal content)
    if (e.target === el.postModal) hidePostDetail();
  });

  // ── React to post (like/dislike) inside the detail modal ───────────────
  el.detailReactions.addEventListener("click", async (e) => {
    const btn = e.target.closest(".btn-reaction");
    if (!btn) return;
    await handleReaction(btn);
  });

  // ── Post a comment ──────────────────────────────────────────────────────
  el.commentForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      await sendComment(e.currentTarget);
      showToast("Comment added");
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Click contact in sidebar → open chat popup with that user ─────────
  el.contactsList.addEventListener("click", async (e) => {
    const item = e.target.closest("[data-contact-id]");
    if (!item) return;
    const id = parseInt(item.dataset.contactId, 10);
    if (!id) return;
    const contact =
      state.contacts.find((c) => c.id === id) ||
      state.contacts.find((c) => String(c.id) === item.dataset.contactId);
    if (!contact) return;
    openPopup();
    await openChat(contact);
  });

  // ── People sidebar search ──────────────────────────────────────────────
  el.contactSearch.addEventListener(
    "input",
    debounce((e) => {
      state.contactSearch = e.target.value;
      renderContacts();
    }, 200)
  );

  // ── Scroll-to-load-more in chat popup ──────────────────────────────────
  el.chatConvMessages.addEventListener("scroll", () => {
    // Load older messages when scrolled to the top
    if (el.chatConvMessages.scrollTop <= 0) {
      loadConversation();
    }
  });

  // ── Chat popup: open / close ───────────────────────────────────────────
  el.chatButton.addEventListener("click", openPopup);
  el.chatPopupClose.addEventListener("click", closePopup);

  // ── Chat popup: back to contact list ───────────────────────────────────
  el.chatConvBack.addEventListener("click", goBackToList);

  // ── Chat popup: tab switching (Recent / All Users) ─────────────────────
  // When a tab is clicked, if a conversation is open, we first go back to
  // the contact list view before switching tabs. Otherwise the user would
  // see no visual change (the list is hidden behind the conversation).

  function switchTab(tab) {
    if (state.activeContact) {
      state.activeContact = null;
      state.messages = [];
      el.chatConversation.classList.add("hidden");
      el.chatPopupList.classList.remove("hidden");
    }
    state.popupTab = tab;
    el.chatTabRecent.classList.toggle("active", tab === "recent");
    el.chatTabAll.classList.toggle("active", tab === "all");
    renderPopupList();
  }
  el.chatTabRecent.addEventListener("click", () => switchTab("recent"));
  el.chatTabAll.addEventListener("click", () => switchTab("all"));

  // ── Chat popup: contact search (instant filter) ───────────────────────
  el.chatPopupSearch.addEventListener("input", (e) => {
    state.popupSearch = e.target.value;
    renderPopupList();
  });

  // ── Chat popup: click contact to open conversation ────────────────────
  el.chatPopupList.addEventListener("click", async (e) => {
    const item = e.target.closest("[data-contact-id]");
    if (!item) return;
    e.stopPropagation();

    // Try to find the contact in our state first
    let contact = state.contacts.find(
      (c) => String(c.id) === item.dataset.contactId
    );
    if (!contact) {
      // Fallback: construct from DOM (shouldn't normally happen)
      const id = parseInt(item.dataset.contactId, 10);
      if (!id) return;
      contact = {
        id,
        nickname: item.querySelector("strong")?.textContent || "Unknown",
        first_name: "",
        last_name: "",
        online: true,
        last_message_at: null,
      };
    }
    try {
      await openChat(contact);
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Send private message (chat form submit) ───────────────────────────
  el.messageForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      await sendMessage(e.currentTarget);
    } catch (err) {
      showToast(err.message, true);
    }
  });
}

// ═════════════════════════════════════════════════════════════════════════════
//  BOOTSTRAP (entry point)
// ═════════════════════════════════════════════════════════════════════════════
//
//  When the DOM is ready, we:
//  1. Bind all event listeners (once).
//  2. Check if there's a valid session cookie (GET /api/me).
//  3. Load categories (always, they're cached client-side).
//  4. If logged in, load posts and contacts.
//  5. If not logged in, stay on the auth view.

async function bootstrap() {
  bindEvents();

  // Check for existing session (cookie)
  await loadMe();

  // Load categories regardless of auth state
  await loadCategories();

  if (state.user) {
    await afterAuthSuccess();
  } else {
    el.connectionStatus.textContent = "🔴 Offline";
    el.connectionStatus.className = "status-pill";
  }
}

// Clean up the WebSocket on page unload
window.addEventListener("beforeunload", disconnectSocket);

// Launch the app when the DOM is ready
window.addEventListener("DOMContentLoaded", bootstrap);
