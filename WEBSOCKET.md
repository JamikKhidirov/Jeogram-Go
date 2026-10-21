# Realtime: Raw WebSocket (`/ws`)

Jeogram доставляет события в реальном времени через **raw WebSocket**.
Это основной транспорт для реалтайм-коммуникации между клиентом и сервером.

---

## 1. Подключение

**URL:** `ws://localhost:8080/ws` (или `wss://` для HTTPS-продакшена)

**Аутентификация:** JWT-токен передаётся либо в заголовке, либо в query-параметре:

```text
ws://localhost:8080/ws?token=<access_token>
```

Или через заголовок:

```
Authorization: Bearer <access_token>
```

> Для браузеров кастомный заголовок на WebSocket поставить нельзя — используйте query-параметр `?token=`. Сервер читает токен и из заголовка, и из query.

### Пример подключения (JavaScript / browser)

```js
const ws = new WebSocket("ws://localhost:8080/ws?token=" + accessToken);

ws.onopen = () => console.log("WebSocket connected");
ws.onmessage = (e) => {
  const event = JSON.parse(e.data);
  console.log("Received:", event.type, event.payload);
};
ws.onclose = () => console.log("WebSocket disconnected");
ws.onerror = (err) => console.error("WebSocket error:", err);
```

### Пример подключения (Node.js, `ws` библиотека)

```js
const WebSocket = require("ws");

const ws = new WebSocket("ws://localhost:8080/ws?token=" + accessToken);

ws.on("open", () => console.log("connected"));
ws.on("message", (data) => {
  const event = JSON.parse(data);
  console.log(event.type, event.payload);
});
```

---

## 2. Формат событий

Каждое событие — это JSON-объект:

```json
{
  "type": "<event_name>",
  "payload": { ... }
}
```

`type` — имя события, `payload` — данные события (разная структура для разных типов).

---

## 3. Входящие события (сервер → клиент)

| Событие | Когда приходит | Содержимое `payload` |
|---------|---------------|----------------------|
| `message.new` | Новое сообщение в чате | `PublicMessage` объект |
| `message.edited` | Сообщение отредактировано | `PublicMessage` объект |
| `message.deleted_for_all` | Сообщение удалено «для всех» | `{ message_id, chat_id }` |
| `message.read` | Сообщение прочитано | `{ chat_id, message_id, user_id }` |
| `message.typing` | Кто-то печатает в чате | `{ chat_id, user_id, typing: true }` |
| `call.started` | Начат аудио/видео звонок | `{ call_id, chat_id, room_name, type }` |
| `call.signal` | WebRTC/SDP сигнал звонка | `{ from, type, chat_id, data }` |
| `call.ended` | Звонок завершён | `{ call_id, chat_id }` |
| `call.participant_muted` | Участник отключил микрофон/камеру | `{ call_id, user_id, kind, muted }` |
| `notification` | Новое in-app уведомление | `Notification` объект |
| `presence` | Изменение онлайн-статуса пользователя | `{ user_id, online }` |
| `user.status` | Изменение статуса (online/dnd/invisible) | `{ user_id, status }` |
| `admin.broadcast` | Рассылка от администратора | `{ title, body }` |
| `auth.code_sent` | Код подтверждения отправлен на email | `{ email, purpose }` |
| `profile.updated` | Профиль пользователя обновлён | `{ user_id, fields, user }` |

---

## 4. Исходящие события (клиент → сервер)

Клиент может отправлять следующие события:

### Индикатор печати

```json
{ "type": "typing", "payload": { "chat_id": "<chat_id>", "typing": true } }
```

### Подтверждение прочтения сообщений

Делается через HTTP (`POST /chats/{chat_id}/read`), сервер сам рассылает `message.read` через WebSocket.

---

## 5. Email-коды подтверждения (через WebSocket)

При регистрации, смене email или сброса пароля сервер отправляет код на email пользователя. Событие `auth.code_sent` приходит через WebSocket, чтобы клиент знал, что код отправлен.

### Регистрация

1. Клиент отправляет `POST /auth/register` с email
2. Сервер создаёт пользователя и отправляет код подтверждения на email
3. Через WebSocket приходит событие:

```json
{
  "type": "auth.code_sent",
  "payload": { "email": "user@example.com", "purpose": "registration" }
}
```

4. Клиент подтверждает код через `POST /auth/verify-email`

### Смена email

1. Клиент отправляет `POST /auth/change-email/request` с новым email
2. Сервер отправляет код на новый email
3. Через WebSocket приходит:

```json
{
  "type": "auth.code_sent",
  "payload": { "email": "new@example.com", "purpose": "email_change" }
}
```

4. Клиент подтверждает через `POST /auth/change-email`

---

## 6. Редактирование профиля (через WebSocket)

При обновлении профиля через HTTP (`PUT /user/profile` или `POST /user/avatar`) сервер рассылает событие `profile.updated` всем подключённым устройствам пользователя:

```json
{
  "type": "profile.updated",
  "payload": { "user_id": "<uuid>", "fields": ["username", "avatar_url"] }
}
```

Это позволяет синхронизировать профиль на всех устройствах в реальном времени.

---

## 7. Звонки и Push-уведомления (LiveKit + WebRTC)

### Создание звонка

1. Клиент отправляет `POST /calls/livekit/start` (или `POST /calls` для WebRTC)
2. Сервер создаёт звонок в базе данных (PostgreSQL)
3. Через LiveKit создаётся комната для аудио/видео звонка
4. Через WebSocket всем участникам чата приходит событие `call.started`:

```json
{
  "type": "call.started",
  "payload": {
    "call_id": "<uuid>",
    "chat_id": "<uuid>",
    "room_name": "jeogram_<call_id>",
    "type": "video",
    "initiator": "<user_id>"
  }
}
```

### Push-уведомления о звонке

При создании звонка сервер отправляет push-уведомления всем участникам чата:

- **Android** — через Firebase Cloud Messaging (FCM)
- **iOS** — через Apple Push Notification service (APNs)

Пуш содержит информацию о звонке, чтобы пользователь мог принять или отклонить его.

### Присоединение к звонку

1. Клиент вызывает `POST /calls/livekit/{id}/join` или `GET /calls/livekit/{id}/token`
2. Сервер возвращает LiveKit JWT-токен и имя комнаты
3. Клиент подключается к LiveKit комнате для аудио/видео связи

### Завершение звонка

1. Инициатор вызывает `POST /calls/livekit/{id}/end`
2. Сервер удаляет LiveKit комнату и завершает звонок в БД
3. Через WebSocket рассылается `call.ended`

### Мьют микрофона/камеры

1. Клиент вызывает `POST /calls/livekit/{id}/mute` с `{ "kind": "audio|video", "muted": true }`
2. Через WebSocket рассылается `call.participant_muted`

---

## 8. Push-уведомления (Android / iOS)

### Регистрация устройства

Клиент регистрирует push-токен через `POST /notifications/device`:

```json
{
  "platform": "ios",
  "token": "<apns_device_token>",
  "device_model": "iPhone 14",
  "os_version": "17.0",
  "app_version": "1.0.0"
}
```

### Типы пуш-уведомлений

| Событие | Когда отправляется | Содержимое |
|---------|-------------------|------------|
| `call.incoming` | Входящий звонок | `call_id`, `chat_id`, `room_name`, `type`, `initiator` |
| `message.new` | Новое сообщение | `chat_id`, `message_id`, `text`, `sender_id` |
| `message.reaction` | Новая реакция | `message_id`, `emoji`, `user_id` |
| `system.alert` | Системное уведомление | `title`, `body` |

### Android (FCM)

Пуш отправляется через Firebase Cloud Messaging. Требуется `FCM_SERVER_KEY` или `FCM_OAUTH_TOKEN` в `.env`.

### iOS (APNs)

Пуш отправляется через Apple Push Notification service. Требуется `APNS_KEY_ID`, `APNS_TEAM_ID`, `APNS_KEY_PATH`, `APNS_BUNDLE_ID` в `.env`.

---

## 9. Админ-рассылка пуш-уведомлений

Администратор может отправить push-уведомление всем онлайн-пользователям через `POST /admin/broadcast`:

```json
{
  "title": "Важное объявление",
  "body": "Приложение будет недоступно с 02:00 до 04:00 по техпричинам"
}
```

Сервер:
1. Сохраняет уведомление в базу данных
2. Рассылает через WebSocket событие `admin.broadcast`
3. Отправляет push-уведомления всем устройствам (Android + iOS)

---

## 10. Тестирование в Postman

1. Импортируйте **оба** файла: сначала окружение
   `postman/Jeogram.postman_environment.json`, затем коллекцию
   `postman/Jeogram API.postman_collection.json`
2. Выберите окружение `Jeogram Local` в правом верхнем углу Postman
3. Войдите: папка `01. Auth` → `POST /auth/register` или `POST /auth/login`
   (токен автоматически сохранится в `{{access_token}}`)
4. Откройте папку `12. Realtime — WebSocket и Socket.IO`:
   - `WS /ws — основной канал realtime` → **Connect** → `ws://localhost:8080/ws?token={{access_token}}`
   - `Socket.IO /socket.io — подключение` → **Connect** → отправьте кадр `40`
   - `WS /calls/ws — сигналинг WebRTC` → сигналинг offer/answer/ice
   - `LiveKit /rtc — сигналинг SFU` → подключение к комнате LiveKit
5. В другой вкладке выполните действие (отправьте сообщение, создайте звонок)
6. В окне WebSocket придёт соответствующее событие в реальном времени

Полный порядок работы по папкам: `00 Старт → 01 Auth → 02 Auth → 03 Users →
04 Contacts → 05 Chats → 06 Messages → 07 Media → 08 Notifications → 09 E2EE →
10 Calls → 11 Calls LiveKit → 12 Realtime → 13 Phone → 14 Admin → 15 Cleanup`.
Идентификаторы (`chat_id`, `message_id`, `call_id`, `livekit_token`) подставляются
автоматически тест-скриптами запросов.

---

## 11. Надёжность

- Сервер шлёт Ping каждые 30 секунд для поддержания соединения
- При обрыве соединения клиент должен переподключиться с тем же токеном
- Все события дублируются в базу данных (PostgreSQL) для надежности
- При отключённом Kafka realtime продолжает работать напрямую через WebSocket-хаб
- Push-уведомления отправляются асинхронно через Kafka-консьюмер (или напрямую если Kafka отключена)

---

## 12. Socket.IO — второй транспорт

Кроме raw WebSocket доступен Socket.IO-транспорт (`/socket.io`, протокол v2 /
Engine.IO v3). События те же самые, подключение:

```text
ws://localhost:8080/socket.io/?token=<access_token>&EIO=3&transport=websocket
```

Подробности (handshake, кадры `40`/`42`, пример на socket.io-client) —
в [SOCKETIO.md](SOCKETIO.md).

---

## 13. LiveKit (SFU) — звонки через WebSocket

Для звонков с SFU-маршрутизацией клиент подключается к самому LiveKit:

```text
ws://{{livekit_host}}/rtc?access_token=<livekit_token>&autoSubscribe=1
```

Токен и имя комнаты берутся из `POST /calls/livekit/{id}/join`
(`token`, `room_name`, `client_url`). В Postman это запросы в папке
`11. Calls LiveKit` и WebSocket `LiveKit /rtc` в папке `12. Realtime`.

