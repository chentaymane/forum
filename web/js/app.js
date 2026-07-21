// Simple SPA helpers: one view visible at a time, small fetch wrapper.

let me = null; // logged in user {id, nickname}

// showToast displays a small, accessible notification without blocking the page.
function showToast(message, type = "error") {
    if (!message) return;
    const region = document.getElementById("toast-region");
    const duplicate = [...region.querySelectorAll(".toast-message")]
        .find((item) => item.textContent === message);
    if (duplicate) return;
    const toast = document.createElement("div");
    toast.className = `toast toast-${type}`;
    toast.setAttribute("role", type === "error" ? "alert" : "status");

    const icon = document.createElement("span");
    icon.className = "toast-icon";
    icon.textContent = type === "error" ? "!" : "i";

    const text = document.createElement("span");
    text.className = "toast-message";
    text.textContent = message;

    const close = document.createElement("button");
    close.className = "toast-close";
    close.type = "button";
    close.setAttribute("aria-label", "Dismiss notification");
    close.textContent = "×";

    toast.append(icon, text, close);
    region.appendChild(toast);

    let timer;
    const dismiss = () => {
        clearTimeout(timer);
        if (!toast.isConnected || toast.classList.contains("toast-out")) return;
        toast.classList.add("toast-out");
        toast.addEventListener("animationend", () => toast.remove(), { once: true });
    };
    close.addEventListener("click", dismiss);
    timer = setTimeout(dismiss, 4500);
}

// Give maxlength fields visible feedback instead of silently rejecting extra text.
document.addEventListener("beforeinput", (e) => {
    const field = e.target;
    if (!(field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement)) return;
    if (field.maxLength < 0 || !e.data || e.inputType.startsWith("delete")) return;
    const selected = Math.abs((field.selectionEnd ?? 0) - (field.selectionStart ?? 0));
    if (field.value.length - selected + e.data.length > field.maxLength) {
        showToast(`This field is limited to ${field.maxLength} characters.`);
    }
}, true);

document.addEventListener("paste", (e) => {
    const field = e.target;
    if (!(field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement)) return;
    if (field.maxLength < 0) return;
    const pasted = e.clipboardData?.getData("text") || "";
    const selected = Math.abs((field.selectionEnd ?? 0) - (field.selectionStart ?? 0));
    if (field.value.length - selected + pasted.length > field.maxLength) {
        showToast(`This field is limited to ${field.maxLength} characters.`);
    }
}, true);

// logoutLocal resets the client state and goes back to the login page.
function logoutLocal() {
    closeChatEverything();
    me = null;
    if (!document.getElementById("auth-view").classList.contains("hidden")) {
        return;
    }
    window.location.reload();
}

// showView hides every .view and shows the requested one.
function showView(name) {
    document.querySelectorAll(".view").forEach((v) => v.classList.add("hidden"));
    document.getElementById(name + "-view").classList.remove("hidden");
    document.getElementById("layout").classList.toggle("hidden", name === "auth" || name === "error");
}

// errorTexts maps a status code to the title and message of the error view.
const errorTexts = {
    400: ["Bad Request", "The request could not be understood."],
    404: ["Not Found", "The page you are looking for does not exist."],
    405: ["Method Not Allowed", "This method is not allowed on this page."],
    500: ["Server Error", "Something went wrong on our side. Please try again later."],
};

// showError fills and shows the error view (part of the single page).
function showError(code) {
    const [title, msg] = errorTexts[code] || ["Error", "An unexpected error occurred."];
    document.getElementById("error-code").textContent = code;
    document.getElementById("error-title").textContent = title;
    document.getElementById("error-msg").textContent = msg;
    showView("error");
}

// api fetches JSON and throws on error responses.
// A 401 while logged in means the session expired: log out.
async function api(path, opts) {
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

// throttle limits how often fn can run (used on the scroll events).
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

// On page load: unknown URLs show the 404 view, otherwise restore the
// session if any or show the login form.
window.addEventListener("DOMContentLoaded", async () => {
    if (location.pathname !== "/") {
        showError(404);
        return;
    }
    try {
        me = await api("/api/me");
        enterForum();
    } catch {
        showView("auth");
    }
});
