// Posts feed, post creation, comments and likes/dislikes.

let posts = [];
let currentPostId = 0;
let filter = ""; // active feed filter, e.g. "mine=1" or "category=2"

const ICON_UP = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M7 10v12"/><path d="M15 5.88 14 10h5.83a2 2 0 0 1 1.92 2.56l-2.33 8A2 2 0 0 1 17.5 22H4a2 2 0 0 1-2-2v-8a2 2 0 0 1 2-2h2.76a2 2 0 0 0 1.79-1.11L12 2h0a3.13 3.13 0 0 1 3 3.88Z"/></svg>`;
const ICON_DOWN = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 14V2"/><path d="M9 18.12 10 14H4.17a2 2 0 0 1-1.92-2.56l2.33-8A2 2 0 0 1 6.5 2H20a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2h-2.76a2 2 0 0 0-1.79 1.11L12 22h0a3.13 3.13 0 0 1-3-3.88Z"/></svg>`;

// loadCategories fills the create post form and the filter bar.
async function loadCategories() {
    const cats = await api("/api/categories");
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

// loadPosts renders the feed with the active filter.
async function loadPosts() {
    posts = await api("/api/posts" + (filter ? "?" + filter : ""));
    const div = document.getElementById("posts");
    div.innerHTML = posts.length ? "" : `<p class="empty">Nothing here yet — write the first post.</p>`;
    posts.forEach((p) => {
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
            </div>`;
        card.onclick = () => openPost(p.id);
        div.appendChild(card);
    });
}

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
        <div class="post-actions">${reactions("post", p)}</div>`;
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
            <div class="post-actions">${reactions("comment", c)}</div>`;
        div.appendChild(el);
    });
}

// Create a post
document.getElementById("post-form").onsubmit = async (e) => {
    e.preventDefault();
    const f = e.target;
    const categories = [...document.querySelectorAll("#post-cats input:checked")].map((i) => +i.value);
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
    await loadPosts(); // refresh the comment counter
    loadComments();
};

// Back to the feed
document.getElementById("back-btn").onclick = () => {
    currentPostId = 0;
    showView("feed");
    loadPosts();
};
