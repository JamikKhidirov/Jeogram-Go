# Realtime: Raw WebSocket (`/ws`)

Jeogram доставляет события в реальном времени двумя способами:

1. **Raw WebSocket** — `ws://<host>/ws` (этот файл). Полностью совместим с вкладкой
   **WebSocket** в Postman, поэтому тестируется «из коробки», без дополнительного ПО.
2. **Socket.IO** — см. [SOCKETIO.md](SOCKETIO.md). Нужен JS-клиент (Node.js/browser),
   так как встроенный WebSocket-клиент Postman НЕ понимает протокол Socket.IO.

Оба транспорта получают **одни и те же** события (`message.new`, `message.read`,
`typing`, `presence`, `notification`, `call.signal`) — сервер фан-аутит каждое
событие в оба хаба через единый `realtime.Broadcaster`.

---

## 1. Подключение

- **URL:** `ws://localhost:8080/ws` (в Docker — `ws://localhost:8080/ws`)
- **Аутентификация:** JWT в заголовке `Authorization: Bearer <access_token>`
  (получить токен через `POST /auth/login`).
- Транспорт: `ws://` для локального/http, `wss://` для https-деплоя.

Пример (browser/Node, `gorilla/websocket`):

```js
const ws = new WebSocket("ws://localhost:8080/ws", {
  headers: { Authorization: "Bearer " + accessToken }
});
ws.onmessage = (e) => console.log(JSON.parse(e.data));
```

> Для браузеров кастомный заголовок на WebSocket поставить нельзя — используйте
> query-параметр: `ws://localhost:8080/ws?token=<access_token>`. Сервер читает
> токен и из заголовка, и из query.

---

## 2. Формат сообщений

Каждое событие — JSON:

```json
{
  "type": "message.new",
  "payload": { "id": "...", "chat_id": "...", "text": "...", "sender_id": "..." }
}
```

### Входящие события (сервер → клиент)

| `type`             | Когда                                  | `payload` (кратко) |
|--------------------|----------------------------------------|--------------------|
| `message.new`      | новое сообщение в чате                 | `PublicMessage`    |
| `message.read`     | кто-то прочитал сообщение              | `{chat_id, message_id, user_id}` |
| `typing`           | участник печатает                      | `{chat_id, user_id}` |
| `presence`         | изменение онлайн-статуса               | `{user_id, online}` |
| `notification`     | новое in-app уведомление              | `Notification`     |
| `call.signal`      | WebRTC-сигнал звонка                   | `{call_id, from, data}` |
| `call.started`     | звонок начат                           | `{call_id, chat_id}` |
| `call.ended`       | звонок завершён                        | `{call_id}`        |

### Исходящие события (клиент → сервер)

Клиент может слать `typing`, чтобы оповестить других:

```json
{ "type": "typing", "payload": { "chat_id": "<chat_id>" } }
```

Остальные действия (отправка сообщения, прочтение, реакции) делаются обычными
**HTTP**-запросами (`POST /messages`, `POST /messages/{id}/read`, …) — сервер сам
разошлёт события всем участникам чата через WebSocket/Socket.IO.

---

## 3. Тестируем в Postman (пошагово)

1. Импортируйте сначала окружение `postman/Jeogram.postman_environment.json`,
   затем коллекцию `postman/Jeogram API.postman_collection.json`
   (`Import` → выберите файлы). В Postman выберите окружение **Jeogram Local**.
2. Откройте папку **Auth → Register**, нажмите **Send** (создаст пользователя).
3. Откройте **Auth → Login**, нажмите **Send**. Скрипт `Tests` автоматически
   сохранит `access_token` в переменную коллекции `{{access_token}}`.
4. Перейдите в папку **Realtime (WebSocket / Socket.IO)** → запрос **WS /ws (raw WebSocket, realtime)**.
5. Postman откроет вкладку **WebSocket**. Убедитесь, что адрес:
   `ws://{{base_url_ws}}/ws?token={{access_token}}` (переменная `base_url_ws` = `ws://localhost:8080`).
6. Нажмите **Connect**. В логе появится `[connection established]`.
7. В **новой вкладке** выполните `POST /messages` (папка Messages) с телом
   `{"chat_id":"{{chat_id}}","text":"привет","type":"text"}`.
8. В окне WebSocket придёт событие `message.new` с этим сообщением — **realtime работает**.

> Подсказка: `chat_id` берётся из ответа `POST /chats/private` или `POST /chats/group`.
> Сохраните его в переменную `{{chat_id}}` вручную (Postman → Variables), либо скопируйте
> из ответа в URL запроса.

---

## 4. Проверка «всё ли работает» без Postman

Самый быстрый smoke-тест — наш Go-скрипт (проверяет и доставку `message.new`,
и новые эндпоинты прочитано/не прочитано/mute/печать):

```bash
go run ./scripts/smoke
```

Ожидаемый результат — 13 строк `[PASS]`, включая
`ws received message.new`, `per-message read -> unread 0`, `mute chat`, `typing indicator`.

---

## 5. Как это устроено (надёжность)

- Единый интерфейс `realtime.Broadcaster` (`internal/pkg/realtime`) фан-аутит событие
  сразу в raw-WS хаб (`ws.Hub`) и в Socket.IO-сервер. Добавить третий транспорт —
  одна строка в `MultiBroadcaster`.
- HTTP-обработчики НЕ зависят от Kafka: сообщение всегда рассылается через хаб
  напрямую, а Kafka-консьюмер лишь дублирует in-app уведомления и push. Если Kafka
  упадёт — realtime продолжает работать.
- `ws.Hub` держит подключения в `map[userID][]conn` и шлёт только адресатам
  (`SendToUsers`), поэтому трафик не дублируется на всех.
