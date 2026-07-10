// Private messages: user list, websocket, chat box with paginated history.

let ws = null;
let online = new Set(); // ids of online users
let unread = new Set(); // ids of users with unseen messages
let openChatId = 0;     // id of the user we are chatting with
let loaded = 0;         // number of messages loaded in the open chat

// initChat connects the websocket and loads the user list.
function initChat() {
    ws = new WebSocket("ws://" + location.host + "/ws");
    ws.onmessage = (e) => {
        const msg = JSON.parse(e.data);
        if (msg.type === "online") {
            online = new Set(msg.users);
            loadUsers();
        } else if (msg.type === "message") {
            onMessage(msg);
        }
    };
    loadUsers();
}

// loadUsers renders the sidebar (server orders by last message, then name).
async function loadUsers() {
    if (!me) return;
    const users = await api("/api/users");
    const ul = document.getElementById("user-list");
    ul.innerHTML = "";
    users.forEach((u) => {
        const li = document.createElement("li");
        li.className = online.has(u.id) ? "online" : "offline";
        li.innerHTML = `<span class="dot"></span>${esc(u.nickname)}` +
            (unread.has(u.id) ? `<span class="badge">new</span>` : "");
        li.onclick = () => openChat(u.id, u.nickname);
        ul.appendChild(li);
    });
}

// onMessage handles an incoming (or echoed) private message.
function onMessage(msg) {
    const other = msg.from === me.id ? msg.to : msg.from;
    if (openChatId === other) {
        const box = document.getElementById("chat-messages");
        box.appendChild(renderMsg(msg));
        box.scrollTop = box.scrollHeight;
        loaded++;
    } else if (msg.from !== me.id) {
        unread.add(other); // notification badge without refreshing
    }
    loadUsers(); // reorder like discord
}

// renderMsg builds one message element: date + nickname + content.
function renderMsg(m) {
    const el = document.createElement("div");
    el.className = "msg" + (m.from === me.id ? " mine" : "");
    el.innerHTML = `<span class="meta">${m.date} &middot; ${esc(m.nickname)}</span><p>${esc(m.content)}</p>`;
    return el;
}

// openChat opens the chat box and loads the last 10 messages.
async function openChat(id, nickname) {
    openChatId = id;
    unread.delete(id);
    loadUsers();

    document.getElementById("chat-name").textContent = nickname;
    document.getElementById("chat-box").classList.remove("hidden");
    const box = document.getElementById("chat-messages");
    box.innerHTML = "";

    const msgs = await api(`/api/messages?with=${id}&offset=0`);
    loaded = msgs.length;
    msgs.forEach((m) => box.appendChild(renderMsg(m)));
    box.scrollTop = box.scrollHeight;
    document.getElementById("chat-input").focus();
}

// loadMore prepends 10 older messages, keeping the scroll position.
async function loadMore() {
    if (!openChatId) return;
    const box = document.getElementById("chat-messages");
    const more = await api(`/api/messages?with=${openChatId}&offset=${loaded}`);
    if (!more.length) return;
    loaded += more.length;

    const oldHeight = box.scrollHeight;
    for (let i = more.length - 1; i >= 0; i--) box.prepend(renderMsg(more[i]));
    box.scrollTop = box.scrollHeight - oldHeight;
}

// Throttled scroll: ask for older messages when reaching the top.
document.getElementById("chat-messages").addEventListener(
    "scroll",
    throttle(() => {
        if (document.getElementById("chat-messages").scrollTop === 0) loadMore();
    }, 1000)
);

// Send a message through the websocket.
document.getElementById("chat-form").onsubmit = (e) => {
    e.preventDefault();
    const input = document.getElementById("chat-input");
    const content = input.value.trim();
    if (!content || !openChatId || !ws) return;
    ws.send(JSON.stringify({ type: "message", to: openChatId, content: content }));
    input.value = "";
};

// Close the chat box.
document.getElementById("chat-close").onclick = () => {
    openChatId = 0;
    document.getElementById("chat-box").classList.add("hidden");
};

// closeChatEverything resets the chat state on logout.
function closeChatEverything() {
    if (ws) {
        ws.close();
        ws = null;
    }
    openChatId = 0;
    online = new Set();
    unread = new Set();
    document.getElementById("chat-box").classList.add("hidden");
    document.getElementById("user-list").innerHTML = "";
}
