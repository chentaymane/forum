// Login / Register / Logout handling.

function showAuthError(msg) {
    const el = document.getElementById("auth-error");
    el.textContent = msg;
    el.classList.toggle("hidden", !msg);
}

// Switch between the login and register forms
document.getElementById("show-register").onclick = (e) => {
    e.preventDefault();
    showAuthError("");
    document.getElementById("login-form").classList.add("hidden");
    document.getElementById("register-form").classList.remove("hidden");
};
document.getElementById("show-login").onclick = (e) => {
    e.preventDefault();
    showAuthError("");
    document.getElementById("register-form").classList.add("hidden");
    document.getElementById("login-form").classList.remove("hidden");
};

// Login with nickname or e-mail + password
document.getElementById("login-form").onsubmit = async (e) => {
    e.preventDefault();
    const f = e.target;
    try {
        me = await api("/api/login", post({
            identifier: f.identifier.value,
            password: f.password.value,
        }));
        f.reset();
        showAuthError("");
        enterForum();
    } catch (err) {
        showAuthError(err.message);
    }
};

// Register a new account (logs the user in directly)
document.getElementById("register-form").onsubmit = async (e) => {
    e.preventDefault();
    const f = e.target;
    try {
        me = await api("/api/register", post({
            nickname: f.nickname.value,
            age: +f.age.value,
            gender: f.gender.value,
            firstName: f.firstName.value,
            lastName: f.lastName.value,
            email: f.email.value,
            password: f.password.value,
        }));
        f.reset();
        showAuthError("");
        enterForum();
    } catch (err) {
        showAuthError(err.message);
    }
};

// Logout works from any view since the navbar is always there
document.getElementById("logout-btn").onclick = async () => {
    try { await api("/api/logout", { method: "POST" }); } catch {}
    authChannel.postMessage("logout");
    logoutLocal();
};
