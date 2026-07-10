// Simple SPA helpers: one view visible at a time, small fetch wrapper.

let me = null; // logged in user {id, nickname}

// showView hides every .view and shows the requested one.
function showView(name) {
    document.querySelectorAll(".view").forEach((v) => v.classList.add("hidden"));
    document.getElementById(name + "-view").classList.remove("hidden");
    document.getElementById("layout").classList.toggle("hidden", name === "auth");
}

// api fetches JSON and throws on error responses.
async function api(path, opts) {
    const res = await fetch(path, opts);
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || "request failed");
    return data;
}

// post builds fetch options for a JSON POST request.
function post(data) {
    return {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
    };
}

// esc escapes HTML to avoid injection.
function esc(s) {
    const d = document.createElement("div");
    d.textContent = s;
    return d.innerHTML;
}

// throttle limits how often fn can run (used on the chat scroll).
function throttle(fn, ms) {
    let last = 0;
    return (...args) => {
        const now = Date.now();
        if (now - last >= ms) {
            last = now;
            fn(...args);
        }
    };
}

// enterForum shows the forum after a successful login/register.
function enterForum() {
    document.getElementById("nav-user").textContent = me.nickname;
    showView("feed");
    loadCategories();
    loadPosts();
    initChat();
}

// On page load: restore the session if any, otherwise show login.
window.addEventListener("DOMContentLoaded", async () => {
    try {
        me = await api("/api/me");
        enterForum();
    } catch {
        showView("auth");
    }
});
