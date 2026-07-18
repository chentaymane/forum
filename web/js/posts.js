// Posts feed, post creation, comments and likes/dislikes.

let posts = [];
let currentPostId = 0;
let filter = "";           // active feed filter, e.g. "mine=1" or "category=2"
let postsDone = false;     // true when the server has no more posts
let loadingPosts = false;  // true while a page request is in flight
let validCatIds = new Set();

const ICON_UP = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M7 10v12"/><path d="M15 5.88 14 10h5.83a2 2 0 0 1 1.92 2.56l-2.33 8A2 2 0 0 1 17.5 22H4a2 2 0 0 1-2-2v-8a2 2 0 0 1 2-2h2.76a2 2 0 0 0 1.79-1.11L12 2h0a3.13 3.13 0 0 1 3 3.88Z"/></svg>`;
const ICON_DOWN = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 14V2"/><path d="M9 18.12 10 14H4.17a2 2 0 0 1-1.92-2.56l2.33-8A2 2 0 0 1 6.5 2H20a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2h-2.76a2 2 0 0 0-1.79 1.11L12 22h0a3.13 3.13 0 0 1-3-3.88Z"/></svg>`;
const ICON_DEL = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg>`;

// loadCategories fills the create post form and the filter bar.
async function loadCategories() {
    const cats = await api("/api/categories");
    validCatIds = new Set(cats.map((c) => c.id));
    document.getElementById("post-cats").innerHTML = cats
        .map((c) => `<label><input type="checkbox" value="${c.id}"> ${esc(c.name)}</label>`)
        .join("");
    document.getElementById("filter-cats").innerHTML = cats
        .map((c) => `<button class="pill" data-filter="category=${c.id}">${esc(c.name)}</button>`)
        .join("");
    resetFilter();
}

// resetFilter goes back to "All" (used on login).
function resetFilter() {
    filter = "";
    document.querySelectorAll(".filter-bar .pill").forEach((p, i) => p.classList.toggle("active", i === 0));
}

// One listener for every filter pill.
document.querySelector(".filter-bar").addEventListener("click", (e) => {
    const pill = e.target.closest(".pill");
    if (!pill) return;
    document.querySelectorAll(".filter-bar .pill").forEach((p) => p.classList.remove("active"));
    pill.classList.add("active");
    filter = pill.dataset.filter;
    loadPosts();
});

// catPills renders category names as pills.
function catPills(categories) {
    if (!categories) return "";
    return `<span class="post-categories">` +
        categories.split(", ").map((c) => `<span class="category">${esc(c)}</span>`).join("") +
        `</span>`;
}

// reactions renders the like/dislike buttons of a post or comment.
function reactions(target, item) {
    return `
        <button class="btn btn-sm btn-like${item.reactedTo === 1 ? " active" : ""}"
            onclick="react(event, '${target}', ${item.id}, 1)">${ICON_UP} ${item.likes}</button>
        <button class="btn btn-sm btn-dislike${item.reactedTo === -1 ? " active" : ""}"
            onclick="react(event, '${target}', ${item.id}, -1)">${ICON_DOWN} ${item.dislikes}</button>`;
}

// react sends a like/dislike and updates the buttons in place (no refresh).
async function react(event, target, id, type) {
    event.stopPropagation(); // do not open the post when liking from the feed
    const body = { type };
    if (target === "post") body.postId = id;
    else body.commentId = id;
    const res = await api("/api/reactions", post(body));

    const actions = event.target.closest(".post-actions");
    const like = actions.querySelector(".btn-like");
    const dislike = actions.querySelector(".btn-dislike");
    like.innerHTML = `${ICON_UP} ${res.likes}`;
    dislike.innerHTML = `${ICON_DOWN} ${res.dislikes}`;
    like.classList.toggle("active", res.reactedTo === 1);
    dislike.classList.toggle("active", res.reactedTo === -1);

    // keep the local cache in sync for the detail view
    if (target === "post") {
        const p = posts.find((x) => x.id === id);
        if (p) Object.assign(p, res);
    }
}

// renderPostCard builds one feed card.
function renderPostCard(p) {
    const card = document.createElement("article");
    card.className = "card post-card";
    card.innerHTML = `
        <div class="post-meta">
            <span class="username">${esc(p.nickname)}</span>
            <span class="date">${p.date}</span>
            ${catPills(p.categories)}
        </div>
        <h3 class="post-title">${esc(p.title)}</h3>
        <p class="post-excerpt">${esc(p.content)}</p>
        <div class="post-actions">
            ${reactions("post", p)}
            <span class="action-stat">${p.comments} comment${p.comments === 1 ? "" : "s"} &rarr;</span>
            ${me && me.id === p.userId ? `<button class="btn btn-sm btn-delete" onclick="deletePost(event, ${p.id})">${ICON_DEL} Delete</button>` : ""}
        </div>`;
    card.onclick = () => openPost(p.id);
    return card;
}

// loadPosts restarts the feed with the active filter (first 10 posts).
async function loadPosts() {
    posts = [];
    postsDone = false;
    document.getElementById("posts").innerHTML = "";
    await loadMorePosts();
    if (!posts.length) {
        document.getElementById("posts").innerHTML =
            `<p class="empty">Nothing here yet — write the first post.</p>`;
    }
}

// loadMorePosts appends the next 10 posts (called by loadPosts and on scroll).
async function loadMorePosts() {
    if (postsDone || loadingPosts) return;
    loadingPosts = true;
    try {
        const page = await api(`/api/posts?offset=${posts.length}` + (filter ? "&" + filter : ""));
        if (page.length < 10) postsDone = true;
        posts = posts.concat(page);
        const div = document.getElementById("posts");
        page.forEach((p) => div.appendChild(renderPostCard(p)));
    } finally {
        loadingPosts = false;
    }
}

// Throttled infinite scroll: fetch the next page when near the bottom of the feed.
window.addEventListener("scroll", throttle(() => {
    if (!me || document.getElementById("feed-view").classList.contains("hidden")) return;
    if (window.innerHeight + window.scrollY >= document.body.offsetHeight - 300) loadMorePosts();
}, 300));

// renderPostDetail fills the detail card of the current post.
function renderPostDetail() {
    const p = posts.find((x) => x.id === currentPostId);
    if (!p) return;
    document.getElementById("post-detail").innerHTML = `
        <div class="post-meta">
            <span class="username">${esc(p.nickname)}</span>
            <span class="date">${p.date}</span>
            ${catPills(p.categories)}
        </div>
        <h2 class="detail-title">${esc(p.title)}</h2>
        <p class="detail-content">${esc(p.content)}</p>
        <div class="post-actions">
            ${reactions("post", p)}
            ${me && me.id === p.userId ? `<button class="btn btn-sm btn-delete" onclick="deletePost(event, ${p.id})">${ICON_DEL} Delete</button>` : ""}
        </div>`;
}

// openPost shows one post with its comments.
function openPost(id) {
    currentPostId = id;
    renderPostDetail();
    showView("post");
    loadComments();
}

// loadComments renders the comments of the open post.
async function loadComments() {
    const comments = await api(`/api/comments?post_id=${currentPostId}`);
    const div = document.getElementById("comments");
    div.innerHTML = comments.length ? "" : `<p class="empty">No comments yet — be the first.</p>`;
    comments.forEach((c) => {
        const el = document.createElement("div");
        el.className = "comment";
        el.innerHTML = `
            <div class="post-meta">
                <span class="username">${esc(c.nickname)}</span>
                <span class="date">${c.date}</span>
            </div>
            <p>${esc(c.content)}</p>
            <div class="post-actions">
                ${reactions("comment", c)}
                ${me && me.id === c.userId ? `<button class="btn btn-sm btn-delete" onclick="deleteComment(event, ${c.id})">${ICON_DEL} Delete</button>` : ""}
            </div>`;
        div.appendChild(el);
    });
}

// Create a post
document.getElementById("post-form").onsubmit = async (e) => {
    e.preventDefault();
    const f = e.target;
    const categories = [...document.querySelectorAll("#post-cats input:checked")]
        .map((i) => +i.value)
        .filter((id) => validCatIds.has(id));
    await api("/api/posts", post({
        title: f.title.value,
        content: f.content.value,
        categories: categories,
    }));
    f.reset();
    loadPosts();
};

// Create a comment
document.getElementById("comment-form").onsubmit = async (e) => {
    e.preventDefault();
    const f = e.target;
    await api("/api/comments", post({ postId: currentPostId, content: f.content.value }));
    f.reset();
    // Bump the cached counter (the feed is redrawn when going back anyway).
    const p = posts.find((x) => x.id === currentPostId);
    if (p) p.comments++;
    loadComments();
};

// Back to the feed
document.getElementById("back-btn").onclick = () => {
    currentPostId = 0;
    showView("feed");
    loadPosts();
};

// Delete a post (owner only)
async function deletePost(event, id) {
    event.stopPropagation();
    if (!await confirmAction("Delete this post?")) return;
    try {
        await api("/api/posts", post({ action: "delete", id: id }));
        posts = posts.filter((p) => p.id !== id);
        const card = event.target.closest(".post-card");
        if (card) card.remove();
        if (currentPostId === id) {
            currentPostId = 0;
            showView("feed");
            loadPosts();
        }
    } catch (e) {
        confirmAction(e.message || "Failed to delete");
    }
}

// Delete a comment (owner only)
async function deleteComment(event, id) {
    event.stopPropagation();
    if (!await confirmAction("Delete this comment?")) return;
    try {
        await api("/api/comments", post({ action: "delete", id: id }));
        const el = event.target.closest(".comment");
        if (el) el.remove();
        const p = posts.find((x) => x.id === currentPostId);
        if (p) p.comments--;
    } catch (e) {
        confirmAction(e.message || "Failed to delete");
    }
}
