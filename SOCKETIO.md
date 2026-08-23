# Realtime: Socket.IO (`/socket.io/`)

Наряду с raw WebSocket сервер поднимает **Socket.IO**-транспорт (библиотека
`github.com/googollee/go-socket.io`). Он нужен, если ваш клиент использует
Socket.IO (например, фронтенд на React/Vue с `socket.io-client`).

> ⚠️ **Важно про совместимость.** Go-сервер реализует протокол **Socket.IO v2**
> (engine.io v3). Поэтому JS-клиент ставьте строго версии **2.x**:
> ```bash
> npm install socket.io-client@2.3.1
> ```
> Клиент `socket.io-client@4` (последний) использует engine.io v4 и **НЕ**
> соединится с этим сервером.

---

## 1. Подключение

```js
// Node.js / browser, ES-стиль
const io = require("socket.io-client");

const socket = io("http://localhost:8080", {
  path: "/socket.io",
  transports: ["websocket"],          // можно ["polling","websocket"]
  query: { token: ACCESS_TOKEN },      // JWT из POST /auth/login
  auth: { token: ACCESS_TOKEN }        // дублируем для надёжности
});

socket.on("connect", () => console.log("connected", socket.id));
socket.on("connect_error", (e) => console.error("connect_error", e.message));
```

Сервер в `OnConnect` проверяет `token` (из query `?token=` или заголовка
`Authorization: Bearer …`), и при успехе сажает сокет в приватную комнату
`u:<userID>`. Невалидный токен → соединение разрывается.

---

## 2. События (те же, что у raw WebSocket)

Клиент подписывается на имена событий:

```js
socket.on("message.new",    (data) => console.log("new msg", data));
socket.on("message.read",   (data) => console.log("read",   data));
socket.on("typing",         (data) => console.log("typing", data));
socket.on("presence",       (data) => console.log("presence", data));
socket.on("notification",   (data) => console.log("notif", data));
socket.on("call.signal",    (data) => console.log("signal", data));
socket.on("call.started",   (data) => console.log("call start", data));
socket.on("call.ended",     (data) => console.log("call end", data));
```

Исходящие (клиент → сервер) — например, «печатаю»:

```js
socket.emit("typing", { chat_id: CHAT_ID });
```

Отправка сообщений/прочтение/реакции — обычными HTTP-запросами; сервер сам
разошлёт события подписчикам.

---

## 3. Готовый тест-скрипт (Node.js)

Сохраните как `scripts/socketio_smoke.js` и запустите:

```bash
npm init -y
npm install socket.io-client@2.3.1
node scripts/socketio_smoke.js
```

```js
// scripts/socketio_smoke.js
const io = require("socket.io-client");
const http = require("http");

const BASE = "http://localhost:8080";

function post(path, body, token) {
  return new Promise((resolve, reject) => {
    const data = JSON.stringify(body);
    const req = http.request(BASE + path, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": token ? "Bearer " + token : "",
        "Content-Length": Buffer.byteLength(data),
      },
    }, (res) => {
      let raw = "";
      res.on("data", (c) => (raw += c));
      res.on("end", () => resolve(JSON.parse(raw || "{}")));
    });
    req.on("error", reject);
    req.write(data);
    req.end();
  });
}

async function main() {
  const u1 = "sockA" + Date.now();
  const u2 = "sockB" + Date.now();
  await post("/auth/register", { username: u1, email: u1 + "@x.io", password: "Passw0rd!23" });
  await post("/auth/register", { username: u2, email: u2 + "@x.io", password: "Passw0rd!23" });
  const l1 = await post("/auth/login", { username: u1, password: "Passw0rd!23" });
  const l2 = await post("/auth/login", { username: u2, password: "Passw0rd!23" });
  const t1 = l1.data.access_token, t2 = l2.data.access_token;
  const uid2 = l2.data.data ? l2.data.data.id : l2.data.user?.id;

  // B подключается по Socket.IO
  const sock = io(BASE, { path: "/socket.io", query: { token: t2 }, auth: { token: t2 } });
  let got = false;
  sock.on("message.new", (p) => { got = true; console.log("[Socket.IO] message.new:", p.text); });

  sock.on("connect", async () => {
    // A создаёт приватный чат и шлёт сообщение -> B должен получить событие
    const chat = await post("/chats/private", { user_id: uid2 }, t1);
    const cid = chat.data.id;
    await post("/messages", { chat_id: cid, text: "hello over socket.io", type: "text" }, t1);
    setTimeout(() => {
      console.log(got ? "RESULT: PASS (Socket.IO received message.new)" : "RESULT: FAIL");
      process.exit(got ? 0 : 1);
    }, 1500);
  });
  sock.on("connect_error", (e) => { console.error("connect_error:", e.message); process.exit(1); });
}
main().catch((e) => { console.error(e); process.exit(1); });
```

---

## 4. Как это устроено

- `server.go` создаёт `socketio.NewServer(nil)`, в `OnConnect` аутентифицирует и
  делает `c.Join("u:"+userID)`, монтирует по `r.Handle("/socket.io/*", io)`.
- Рассылка идёт через `realtime.SocketIOBroadcaster`, который для каждого
  получателя вызывает `io.BroadcastToRoom("/", "u:"+userID, type, payload)`.
- `MultiBroadcaster` одновременно пушит в raw-WS и Socket.IO, поэтому неважно,
  каким транспортом подключён получатель.
