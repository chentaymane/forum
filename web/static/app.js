// ═══════════════════════════════════════════════════════════════════════════
//  Forum — Single Page Application (Client)
// ═══════════════════════════════════════════════════════════════════════════
//
// This file handles EVERYTHING the user sees and interacts with:
//   • Page rendering (login, register, feed, post detail, chat)
//   • All API calls to the Go backend (fetch/JSON)
//   • WebSocket connection for real‑time private messages & presence
//   • Client‑side routing via show/hide of view containers
//
// ─── Architecture ─────────────────────────────────────────────────────────
// The state object holds all application data.  When state changes the
// corresponding render*() function is called to update the DOM.  Events
// are bound once at bootstrap – no virtual DOM, just direct DOM manipulation
// which is simple and fast for an app of this size.

// ─── Global State ─────────────────────────────────────────────────────────
const state = {
  user: null,              // Current logged-in user (APIUser object)
  categories: [],          // Available post categories
  posts: [],               // Current feed of posts
  activePost: null,        // Post being viewed in detail panel
  contacts: [],            // All users (except current)
  activeContact: null,     // Currently selected chat partner
  messages: [],            // Messages for the active conversation
  seenMessageIds: new Set(), // Dedup set for real‑time incoming messages
  oldestMessageId: null,   // ID of the oldest loaded message (for pagination)
  hasMoreMessages: true,   // Whether there are more messages to load
  loadingMessages: false,  // Prevents concurrent loadMore calls
  socket: null,            // WebSocket connection
  contactSearch: "",       // Left sidebar search text
  popupSearch: "",         // Chat popup search text
  popupTab: "recent",      // "recent" | "all"
  popupOpen: false,        // Whether the popup is visible
};

// ─── DOM References ───────────────────────────────────────────────────────
// Cached once so we avoid repeated document.getElementById lookups.
const $ = (id) => document.getElementById(id);

const el = {
  authView:         $("auth-view"),
  appView:          $("app-view"),
  showRegister:     $("show-register"),
  showLogin:        $("show-login"),
  registerCard:     $("register-card"),
  logoutButton:     $("logout-button"),
  userBadge:        $("user-badge"),
  connectionStatus: $("connection-status"),
  toast:            $("toast"),
  loginForm:        $("login-form"),
  registerForm:     $("register-form"),
  postForm:         $("post-form"),
  postsList:        $("posts-list"),
  categoryFilter:   $("category-filter"),
  categoryList:     $("category-list"),
  refreshPosts:     $("refresh-posts"),
  postModal:        $("post-modal"),
  detailTitle:      $("detail-title"),
  detailMeta:       $("detail-meta"),
  detailCategories: $("detail-categories"),
  detailBody:       $("detail-content"),
  detailReactions:  $("detail-reactions"),
  commentsList:     $("comments-list"),
  commentForm:      $("comment-form"),
  closeDetail:      $("close-detail"),
  // People sidebar
  contactsList:     $("contacts-list"),
  contactSearch:    $("contact-search"),
  // Chat popup
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

// ═══════════════════════════════════════════════════════════════════════════
//  UTILITY FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════

// --- showToast -----------------------------------------------------------
// Displays a brief notification in the bottom‑right corner.
// Automatically hides after 2.5 seconds.  Pass isError=true for red styling.
function showToast(message, isError = false) {
  el.toast.textContent = message;
  el.toast.classList.remove("hidden", "toast-error");
  if (isError) el.toast.classList.add("toast-error");
  clearTimeout(showToast._timer);
  showToast._timer = setTimeout(() => el.toast.classList.add("hidden"), 2500);
}

// --- setAuthenticated ----------------------------------------------------
// Toggles between the auth view and the app view.
function setAuthenticated(yes) {
  el.authView.classList.toggle("hidden", yes);
  el.appView.classList.toggle("hidden", !yes);
  el.logoutButton.classList.toggle("hidden", !yes);
  el.userBadge.classList.toggle("hidden", !yes);
  el.chatButton.classList.toggle("hidden", !yes);
}

// --- api -----------------------------------------------------------------
// Thin wrapper around fetch() that sends credentials and handles errors.
// Returns the parsed JSON body on success, throws on HTTP errors.
async function api(path, options = {}) {
  const response = await fetch(path, {
    credentials: "same-origin",
    ...options,
  });

  const ct = response.headers.get("content-type") || "";
  const body = ct.includes("application/json") ? await response.json() : await response.text();

  if (!response.ok) {
    const msg = typeof body === "string" ? body : body.error || response.statusText;
    throw new Error(msg);
  }

  return body;
}

// --- escapeHtml -----------------------------------------------------------
// Prevents XSS by encoding HTML special characters.
function escapeHtml(value) {
  const map = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };
  return String(value).replace(/[&<>"']/g, (ch) => map[ch]);
}

// --- debounce & throttle --------------------------------------------------
// Used to limit expensive operations like search filtering and scroll events.

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
      last = now; fn(...args); return;
    }
    if (timer) return;
    timer = setTimeout(() => {
      last = Date.now(); timer = null; fn(...args);
    }, remaining);
  };
}

// ═══════════════════════════════════════════════════════════════════════════
//  RENDER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════
// Each render* function reads from the global state object and updates the
// corresponding section of the DOM.  They are called whenever the relevant
// state changes.

// --- renderCategories ----------------------------------------------------
// Populates the category filter dropdown AND the checkbox chips in the
// new‑post form.
function renderCategories() {
  const filterOpts = [`<option value="">All categories</option>`];
  const chips = [];

  for (const cat of state.categories) {
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

// --- renderPosts ----------------------------------------------------------
// Renders the post feed (list of post cards).
// --- reactionButtons -------------------------------------------------------
// Returns HTML for the like/dislike button pair.
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

// --- handleReaction --------------------------------------------------------
// Sends a like/dislike toggle request to the server and updates the UI.
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
    // Reload the post to get updated counts
    if (state.activePost && String(state.activePost.id) === postId) {
      await loadPost(postId);
    } else {
      // Refresh the feed to update counts in cards
      await loadPosts();
    }
  } catch (err) {
    showToast(err.message, true);
  } finally {
    btn.disabled = false;
  }
}

function renderPosts() {
  if (!state.posts.length) {
    el.postsList.innerHTML = `<p class="empty-state">No posts yet. Be the first to write one!</p>`;
    return;
  }

  el.postsList.innerHTML = state.posts.map((post) => `
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
  `).join("");
}

// --- renderContacts -------------------------------------------------------
// Shows the people sidebar with online/offline indicators.
// Filtered by the current state.contactSearch text.
function renderContacts() {
  const filter = state.contactSearch.trim().toLowerCase();
  const contacts = !filter
    ? state.contacts
    : state.contacts.filter((c) =>
        c.nickname.toLowerCase().includes(filter) ||
        (c.first_name || "").toLowerCase().includes(filter) ||
        (c.last_name || "").toLowerCase().includes(filter)
      );

  if (!contacts.length) {
    el.contactsList.innerHTML = `<p class="empty-state">No users found</p>`;
    return;
  }

  el.contactsList.innerHTML = contacts.map((c) => {
    const active = state.activeContact && state.activeContact.id === c.id;
    const fullName = [c.first_name, c.last_name].filter(Boolean).join(" ");
    const heading = fullName ? `${escapeHtml(c.nickname)} — ${escapeHtml(fullName)}` : escapeHtml(c.nickname);
    return `
      <button type="button" class="contact-item ${active ? "active" : ""}" data-contact-id="${c.id}">
        <span>
          <strong>${heading}</strong>
          <small>${c.last_message_at ? escapeHtml(c.last_message_at) : "No messages yet"}</small>
        </span>
        <span class="status-dot ${c.online ? "online" : "offline"}">${c.online ? "Online" : "Offline"}</span>
      </button>
    `;
  }).join("");
}

// --- renderPostDetail -----------------------------------------------------
// Shows the expanded view of a single post with its full comment thread.
function renderPostDetail(post) {
  state.activePost = post;
  el.postModal.classList.remove("hidden");
  el.detailTitle.textContent = post.title;
  el.detailMeta.textContent = `${post.nickname} · ${post.created_at}`;
  el.detailBody.textContent = post.content;
  el.detailCategories.innerHTML = formatCategories(post.categories || []);
  el.detailReactions.innerHTML = reactionButtons(post);
  el.commentsList.innerHTML = (post.comments || []).map((c) => `
    <article class="comment-card">
      <div class="card-topline">
        <span>${escapeHtml(c.nickname)}</span>
        <span>${escapeHtml(c.created_at)}</span>
      </div>
      <p>${escapeHtml(c.content)}</p>
    </article>
  `).join("") || `<p class="empty-state">No comments yet.</p>`;
}

// --- hidePostDetail -------------------------------------------------------
// Collapses the post detail modal.
function hidePostDetail() {
  state.activePost = null;
  el.postModal.classList.add("hidden");
  el.commentForm.reset();
}

// --- renderConversation ----------------------------------------------------
// Renders the conversation in the chat popup.
function renderConversation() {
  const list = el.chatConvMessages;
  list.innerHTML = state.messages.map((msg) => {
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
  }).join("") || `<p class="empty-state">No messages yet. Say hello!</p>`;
  list.scrollTop = list.scrollHeight;
}

// --- upsertMessage --------------------------------------------------------
// Adds a message to the local state, avoiding duplicates via seenMessageIds.
// Returns true if the message was new.
function upsertMessage(message, prepend = false) {
  if (state.seenMessageIds.has(message.id)) return false;
  state.seenMessageIds.add(message.id);
  if (prepend) state.messages.unshift(message);
  else state.messages.push(message);
  return true;
}

// ─── Chat Popup ─────────────────────────────────────────────────────────────

// --- userItem -------------------------------------------------------------
// Returns HTML for a user entry used in both the left sidebar and popup.
function userItem(c, active) {
  const fullName = [c.first_name, c.last_name].filter(Boolean).join(" ");
  const heading = fullName ? `${escapeHtml(c.nickname)} — ${escapeHtml(fullName)}` : escapeHtml(c.nickname);
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

// --- renderPopupList ------------------------------------------------------
// Renders the chat popup contact list based on the active tab and search.
function renderPopupList() {
  const filter = state.popupSearch.trim().toLowerCase();
  let contacts = state.contacts;
  if (state.popupTab === "recent") {
    contacts = contacts.filter((c) => c.last_message_at);
  }
  if (filter) {
    contacts = contacts.filter((c) =>
      c.nickname.toLowerCase().includes(filter) ||
      (c.first_name || "").toLowerCase().includes(filter) ||
      (c.last_name || "").toLowerCase().includes(filter)
    );
  }
  // Recent tab: order by last_message_at DESC; All tab: alphabetical
  if (state.popupTab === "recent") {
    contacts = [...contacts].sort((a, b) => {
      if (!a.last_message_at) return 1;
      if (!b.last_message_at) return -1;
      return b.last_message_at.localeCompare(a.last_message_at);
    });
  } else {
    contacts = [...contacts].sort((a, b) => a.nickname.localeCompare(b.nickname));
  }
  el.chatPopupList.innerHTML = !contacts.length
    ? `<p class="empty-state">No users found</p>`
    : contacts.map((c) => userItem(c, false)).join("");
}

// --- openPopup ------------------------------------------------------------
function openPopup() {
  state.popupOpen = true;
  el.chatPopup.classList.remove("hidden");
  el.chatButton.classList.add("hidden");
  // Reset to list view
  el.chatConversation.classList.add("hidden");
  el.chatPopupList.classList.remove("hidden");
  if (state.activeContact) {
    state.activeContact = null;
    state.messages = [];
  }
  renderPopupList();
}

// --- closePopup -----------------------------------------------------------
function closePopup() {
  state.popupOpen = false;
  el.chatPopup.classList.add("hidden");
  el.chatButton.classList.remove("hidden");
  state.activeContact = null;
  state.messages = [];
}

// --- goBackToList ---------------------------------------------------------
function goBackToList() {
  state.activeContact = null;
  state.messages = [];
  state.popupSearch = "";
  el.chatPopupSearch.value = "";
  el.chatConversation.classList.add("hidden");
  el.chatPopupList.classList.remove("hidden");
  renderPopupList();
}

// --- formatCategories -----------------------------------------------------
function formatCategories(categories) {
  return categories.map((name) => `<span class="chip">${escapeHtml(name)}</span>`).join("");
}

// ═══════════════════════════════════════════════════════════════════════════
//  DATA LOADING
// ═══════════════════════════════════════════════════════════════════════════

// --- loadMe ---------------------------------------------------------------
// Checks if the user has a valid session cookie.  Called on every page load.
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

// --- loadCategories -------------------------------------------------------
async function loadCategories() {
  const data = await api("/api/categories");
  state.categories = data.categories || [];
  renderCategories();
}

// --- loadPosts ------------------------------------------------------------
// Uses a request‑sequence counter to avoid stale responses overwriting
// newer ones (race condition when the user changes the filter rapidly).
let _loadPostsSeq = 0;
async function loadPosts() {
  const seq = ++_loadPostsSeq;
  const catId = el.categoryFilter.value;
  const suffix = catId ? `?category_id=${encodeURIComponent(catId)}` : "";
  const data = await api(`/api/posts${suffix}`);
  if (seq !== _loadPostsSeq) return; // stale response – discard
  state.posts = data.posts || [];
  renderPosts();
}

// --- loadContacts ---------------------------------------------------------
let _loadContactsSeq = 0;
async function loadContacts() {
  if (!state.user) return;
  const seq = ++_loadContactsSeq;
  const data = await api("/api/chat/contacts");
  if (seq !== _loadContactsSeq) return; // stale response – discard
  state.contacts = data.contacts || [];
  renderContacts();
  // Keep popup list in sync when contacts change
  if (state.popupOpen && !state.activeContact) renderPopupList();
}

// --- loadPost -------------------------------------------------------------
async function loadPost(postId) {
  const data = await api(`/api/posts/${postId}`);
  renderPostDetail(data.post);
}

// --- openChat -------------------------------------------------------------
// Opens the popup conversation with the given contact.
async function openChat(contact) {
  state.activeContact = contact;
  state.messages = [];
  state.seenMessageIds = new Set();
  state.oldestMessageId = null;
  state.hasMoreMessages = true;
  // Show conversation view inside popup
  el.chatConversation.classList.remove("hidden");
  el.chatPopupList.classList.add("hidden");
  el.chatConvUser.textContent = `${contact.nickname} · ${contact.online ? "🟢 Online" : "⚪ Offline"}`;
  el.messageForm.receiver_id.value = contact.id;
  renderConversation();
  await loadConversation();
  await loadContacts(); // refresh presence
}

// --- loadConversation -----------------------------------------------------
// Loads messages (up to 10) for the active conversation.
// Pass beforeId for pagination ("load older messages").
async function loadConversation(beforeId = 0, preserveScroll = false) {
  if (!state.activeContact || state.loadingMessages) return;
  state.loadingMessages = true;

  try {
    const params = new URLSearchParams({ with: String(state.activeContact.id) });
    if (beforeId > 0) params.set("before_id", String(beforeId));

    const data = await api(`/api/chat/messages?${params}`);
    const messages = data.messages || [];

    if (beforeId > 0) {
      // Prepend older messages
      for (const msg of messages) upsertMessage(msg, true);
    } else {
      // Fresh load
      state.messages = [];
      state.seenMessageIds = new Set();
      for (const msg of messages) upsertMessage(msg, false);
    }

    state.oldestMessageId = state.messages.length ? state.messages[0].id : null;
    state.hasMoreMessages = messages.length === 10;
    renderConversation();
  } finally {
    state.loadingMessages = false;
  }
}

// --- handleMessageScroll (throttled) --------------------------------------
// Implements infinite scroll for chat messages: when the user scrolls to
// the top of the message list, load 10 more.
const handleMessageScroll = throttle(() => {
  if (!state.activeContact || !state.hasMoreMessages || state.loadingMessages) return;
  const list = el.chatConvMessages;
  if (list.scrollTop <= 40 && state.oldestMessageId) {
    loadConversation(state.oldestMessageId);
  }
}, 200);

// ═══════════════════════════════════════════════════════════════════════════
//  ACTIONS (form submissions)
// ═══════════════════════════════════════════════════════════════════════════

async function sendMessage(form) {
  const data = await api("/api/messages", {
    method: "POST",
    body: new FormData(form),
  });
  if (data.message) {
    upsertMessage(data.message, false);
    state.oldestMessageId = state.messages.length ? state.messages[0].id : null;
    renderConversation();
  }
  form.reset();
  form.receiver_id.value = state.activeContact.id;
  await loadContacts();
}

async function sendComment(form) {
  if (!state.activePost) return;
  await api(`/api/posts/${state.activePost.id}/comments`, {
    method: "POST",
    body: new FormData(form),
  });
  form.reset();
  await loadPost(state.activePost.id);
  await loadPosts();
}

async function sendPost(form) {
  await api("/api/posts", {
    method: "POST",
    body: new FormData(form),
  });
  form.reset();
  state.activePost = null;
  hidePostDetail();
  await loadPosts();
}

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

async function afterAuthSuccess() {
  await Promise.all([loadPosts(), loadContacts()]);
  hidePostDetail();
  if (state.contacts.length) renderContacts();

  // Periodically refresh contacts so new users appear
  if (window._contactsRefreshTimer) clearInterval(window._contactsRefreshTimer);
  window._contactsRefreshTimer = setInterval(async () => {
    if (state.user) try { await loadContacts(); } catch {}
  }, 30000);
}

// ═══════════════════════════════════════════════════════════════════════════
//  WEBSOCKET (real‑time messaging & presence)
// ═══════════════════════════════════════════════════════════════════════════

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

  // --- onmessage: handle incoming WebSocket events -----------------------
  state.socket.onmessage = async (event) => {
    const payload = JSON.parse(event.data);

    // Presence update – someone came online or went offline
    if (payload.type === "presence") {
      const onlineSet = new Set(payload.online_user_ids || []);
      state.contacts = state.contacts.map((c) => ({ ...c, online: onlineSet.has(c.id) }));
      renderContacts();
      if (state.activeContact) {
        const updated = state.contacts.find((c) => c.id === state.activeContact.id);
        if (updated) state.activeContact = updated;
        el.chatConvUser.textContent = `${state.activeContact.nickname} · ${state.activeContact.online ? "🟢 Online" : "⚪ Offline"}`;
        renderConversation();
      }
      return;
    }

    // New private message
    if (payload.type === "message" && payload.message) {
      const msg = payload.message;
      const activeId = state.activeContact ? state.activeContact.id : null;
      const involved = state.user && (msg.sender_id === state.user.id || msg.receiver_id === state.user.id);

      if (involved) {
        // If no chat is open, just refresh contacts to show latest message
        if (!state.activeContact) {
          await loadContacts();
          return;
        }

        const matchesChat = msg.sender_id === activeId || msg.receiver_id === activeId;
        if (matchesChat) {
          if (upsertMessage(msg, false)) {
            state.oldestMessageId = state.messages.length ? state.messages[0].id : null;
            renderConversation();
          }
        }
        await loadContacts();
      }
    }
  };
}

function disconnectSocket() {
  if (state.socket) {
    state.socket.close();
    state.socket = null;
  }
}

// ═══════════════════════════════════════════════════════════════════════════
//  EVENT BINDING
// ═══════════════════════════════════════════════════════════════════════════

function bindEvents() {
  // ── Auth form toggles ──────────────────────────────────────────────
  // Get references to both auth cards
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

  // ── Login ──────────────────────────────────────────────────────────
  el.loginForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      await submitLogin(e.currentTarget);
      showToast("Welcome back!");
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Register ───────────────────────────────────────────────────────
  el.registerForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      await submitRegister(e.currentTarget);
      showToast("Account created!");
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Logout ─────────────────────────────────────────────────────────
  el.logoutButton.addEventListener("click", async () => {
    try {
      await logout();
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Create post ────────────────────────────────────────────────────
  el.postForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      await sendPost(e.currentTarget);
      showToast("Post published!");
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Refresh posts ──────────────────────────────────────────────────
  el.refreshPosts.addEventListener("click", async () => {
    try {
      await loadPosts();
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Category filter ────────────────────────────────────────────────
  el.categoryFilter.addEventListener("change", async () => {
    try {
      await loadPosts();
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Click post to view detail ──────────────────────────────────────
  // Delegated reaction button handler (must run before the post‑card handler
  // to avoid the card handler consuming the event).
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

  // ── Close detail (button or backdrop click) ────────────────────────
  el.closeDetail.addEventListener("click", hidePostDetail);
  el.postModal.addEventListener("click", (e) => {
    if (e.target === el.postModal) hidePostDetail();
  });

  // ── React to post (detail panel) ───────────────────────────────────
  el.detailReactions.addEventListener("click", async (e) => {
    const btn = e.target.closest(".btn-reaction");
    if (!btn) return;
    await handleReaction(btn);
  });

  // ── Post comment ───────────────────────────────────────────────────
  el.commentForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      await sendComment(e.currentTarget);
      showToast("Comment added");
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Click contact → open popup with conversation ───────────────────
  el.contactsList.addEventListener("click", async (e) => {
    const item = e.target.closest("[data-contact-id]");
    if (!item) return;
    let contact = state.contacts.find((c) => String(c.id) === item.dataset.contactId);
    if (!contact) {
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
      openPopup();
      await openChat(contact);
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Send private message ───────────────────────────────────────────
  el.messageForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      await sendMessage(e.currentTarget);
      showToast("Message sent");
    } catch (err) {
      showToast(err.message, true);
    }
  });

  // ── Infinite scroll for chat messages (popup) ──────────────────────
  el.chatConvMessages.addEventListener("scroll", handleMessageScroll);

  // ── Chat popup: toggle open/close ──────────────────────────────────
  el.chatButton.addEventListener("click", openPopup);
  el.chatPopupClose.addEventListener("click", closePopup);

  // ── Chat popup: back to contact list ───────────────────────────────
  el.chatConvBack.addEventListener("click", goBackToList);

  // ── Chat popup: tab switching (Recent / All Users) ────────────────
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

  // ── Chat popup: contact search (instant) ──────────────────────────
  el.chatPopupSearch.addEventListener("input", (e) => {
    state.popupSearch = e.target.value;
    renderPopupList();
  });

  // ── Chat popup: click contact to open conversation ────────────────
  el.chatPopupList.addEventListener("click", async (e) => {
    const item = e.target.closest("[data-contact-id]");
    if (!item) return;
    e.stopPropagation();
    let contact = state.contacts.find((c) => String(c.id) === item.dataset.contactId);
    if (!contact) {
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
}

// ═══════════════════════════════════════════════════════════════════════════
//  BOOTSTRAP
// ═══════════════════════════════════════════════════════════════════════════

async function bootstrap() {
  // Wire up all DOM events once
  bindEvents();

  // Check if we have a valid session (cookie)
  await loadMe();

  // Load categories regardless (they're cached client‑side)
  await loadCategories();

  if (state.user) {
    // Logged in → load data and show the app
    await afterAuthSuccess();
  } else {
    // Not logged in → show auth view
    el.connectionStatus.textContent = "🔴 Offline";
    el.connectionStatus.className = "status-pill";
  }
}

// Clean up the WebSocket on page unload
window.addEventListener("beforeunload", disconnectSocket);

// Launch the application when the DOM is ready
window.addEventListener("DOMContentLoaded", bootstrap);
