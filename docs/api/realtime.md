# Realtime: WebSocket и Socket.IO

Сервер поддерживает **два** транспорта для одних и тех же событий:
1. **Raw WebSocket** — `ws://localhost:8080/ws` (рекомендуется, легко тестировать в Postman)
2. **Socket.IO** — `http://localhost:8080/socket.io/` (для клиентов на `socket.io-client@2.x`)

Функционально они эквивалентны — используйте то, что удобнее для клиента.

---

## WebSocket (`/ws`)

**Подключение:**

```
ws://localhost:8080/ws?token=<access_token>
```

Аутентификация: заголовок `Authorization: Bearer <token>` либо query-параметр `?token=`.

Подробная документация со всеми событиями: [WEBSOCKET.md](WEBSOCKET.md).

---

## Socket.IO (`/socket.io/`)

```
http://localhost:8080/socket.io/?token=<access_token>&EIO=4&transport=websocket
```

Клиент должен быть строго версии **2.x**:
```bash
npm install socket.io-client@2.3.1
```

События идентичны WebSocket. Каждый пользователь подключается в приватную комнату `u:<userID>`.

---

## События в реальном времени

| Событие | Когда | Канал | Push |
|---------|-------|-------|------|
| `message.new` | Новое сообщение | WebSocket + Socket.IO | ✅ FCM + APNs |
| `message.edited` | Редактирование | WebSocket + Socket.IO | ❌ |
| `message.read` | Прочтение | WebSocket + Socket.IO | ❌ |
| `call.started` | Начат звонок | WebSocket + Socket.IO | ✅ FCM + APNs |
| `call.signal` | WebRTC/SDP сигнал | WebSocket + Socket.IO | ❌ |
| `call.participant_muted` | Мьют участника | WebSocket + Socket.IO | ❌ |
| `call.ended` | Завершён звонок | WebSocket + Socket.IO | ❌ |
| `notification` | Новое уведомление | WebSocket + Socket.IO | ✅ FCM + APNs |
| `presence` | Онлайн-статус | WebSocket + Socket.IO | ❌ |
| `user.status` | Статус пользователя | WebSocket + Socket.IO | ❌ |
| `admin.broadcast` | Админ-рассылка | WebSocket + Socket.IO | ✅ FCM + APNs |
| `auth.code_sent` | Код на email | WebSocket + Socket.IO | ❌ |
| `profile.updated` | Обновлён профиль | WebSocket + Socket.IO | ❌ |

---

## Push-уведомления

Все push-уведомления отправляются через:
- **Android** — Firebase Cloud Messaging (FCM)
- **iOS** — Apple Push Notification service (APNs)

Настройка: [notifications.md](notifications.md)

---

## Звонки (LiveKit + WebRTC)

### LiveKit (аудио/видео звонки с записью)

- `POST /calls/livekit/start` — создать звонок (создаёт LiveKit комнату)
- `POST /calls/livekit/{id}/end` — завершить звонок
- `POST /calls/livekit/{id}/join` — присоединиться к групповому звонку
- `GET /calls/livekit/{id}/token` — получить LiveKit JWT-токен
- `POST /calls/livekit/{id}/mute` — мьют микрофона/камеры
- `POST /calls/livekit/{id}/record/start` — запустить запись
- `POST /calls/livekit/{id}/record/stop` — остановить запись
- `GET /calls/livekit/{id}/recording` — получить ссылку на запись
- `GET /calls/livekit/rooms` — список активных комнат
- `GET /calls/livekit/ice-servers` — STUN/TURN серверы

### WebRTC-сигналинг

- `WS /calls/ws` — WebSocket для обмена SDP/ICE между участниками

### Админ-рассылка

- `POST /admin/broadcast` — отправить уведомление всем онлайн-пользователям
