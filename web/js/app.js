// Simple SPA helpers: one view visible at a time, small fetch wrapper.

let me = null; // logged in user {id, nickname}
const authChannel = new BroadcastChannel("forum-auth");
let sessionCheckInterval = null;
let expectedToken = null; // stored rtf_check value to detect cookie tampering

function getCookie(name) {
    const match = document.cookie.match(new RegExp("(^| )" + name + "=([^;]*)"));
    return match ? decodeURIComponent(match[2]) : null;
}

function logoutLocal() {
    clearInterval(sessionCheckInterval);
    expectedToken = null;
    closeChatEverything();
    me = null;
    if (!document.getElementById("auth-view").classList.contains("hidden")) {
        return;
    }
    window.location.reload();
}

authChannel.onmessage = (e) => {
    if (e.data === "logout") logoutLocal();
};

// showView hides every .view and shows the requested one.
function showView(name) {
    document.querySelectorAll(".view").forEach((v) => v.classList.add("hidden"));
    document.getElementById(name + "-view").classList.remove("hidden");
    document.getElementById("layout").classList.toggle("hidden", name === "auth");
}

// api fetches JSON and throws on error responses.
async function api(path, opts) {
    if (me && getCookie("rtf_check") !== expectedToken) {
        logoutLocal();
        throw new Error("session expired");
    }
    const res = await fetch(path, opts);
    if (res.status === 401 && me) {
        logoutLocal();
        throw new Error("session expired");
    }
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

// confirm shows a modal and returns true if the user clicks Confirm.
function confirmAction(msg) {
    return new Promise((resolve) => {
        const overlay = document.getElementById("modal-overlay");
        document.getElementById("modal-msg").textContent = msg;
        overlay.classList.remove("hidden");
        const yes = () => { cleanup(); resolve(true); };
        const no  = () => { cleanup(); resolve(false); };
        const cleanup = () => {
            overlay.classList.add("hidden");
            document.getElementById("modal-confirm").removeEventListener("click", yes);
            document.getElementById("modal-cancel").removeEventListener("click", no);
            overlay.removeEventListener("click", backdrop);
        };
        const backdrop = (e) => { if (e.target === overlay) no(); };
        document.getElementById("modal-confirm").addEventListener("click", yes);
        document.getElementById("modal-cancel").addEventListener("click", no);
        overlay.addEventListener("click", backdrop);
    });
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
    expectedToken = getCookie("rtf_check");
    document.getElementById("nav-user").textContent = me.nickname;
    showView("feed");
    loadCategories();
    loadPosts();
    initChat();
    clearInterval(sessionCheckInterval);
    sessionCheckInterval = setInterval(async () => {
        try { await api("/api/me"); }
        catch {
            clearInterval(sessionCheckInterval);
            logoutLocal();
        }
    }, 30000);
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
