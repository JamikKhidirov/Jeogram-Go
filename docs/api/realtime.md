# Realtime: Raw WebSocket

Сервер поддерживает **один** транспорт для событий в реальном времени — **raw WebSocket**. Socket.IO удалён.

## WebSocket (`/ws`)

Подключение (raw WebSocket):

```
ws://localhost:8080/ws?token=<access_token>
```

Аутентификация: заголовок `Authorization: Bearer <token>` либо query-параметр `?token=`. После подключения сервер шлёт Ping каждые 30с. Клиент может просто держать соединение — это обеспечивает синхронизацию между всеми устройствами пользователя (один пользователь может быть подключён с телефона и десктопа одновременно).

Входящее событие (JSON):

```json
{ "type": "<event>", "payload": { ... } }
```

## LiveKit звонки

LiveKit предоставляет собственную систему realtime для звонков через WebRTC. Клиент получает JWT-токен через `GET /calls/livekit/{id}/token` и подключается к LiveKit напрямую. Серверные события (`call.started`, `call.ended`, `call.participant_muted`) рассылаются через WebSocket-хаб.

## События

| type | Когда | payload |
| --- | --- | --- |
| `message.new` | новое сообщение | публичное сообщение |
| `message.edited` | редактирование | публичное сообщение |
| `message.deleted_for_all` | удаление «для всех» | `{message_id, chat_id}` |
| `message.typing` | индикатор печати | `{chat_id, user_id, typing}` |
| `message.read` | прочтение | `{chat_id, user_id}` |
| `call.started` | начало звонка | запись звонка |
| `call.signal` | WebRTC-сигналинг | `{from, type, chat_id, data}` |
| `call.participant_muted` | мьют участника | `{call_id, user_id, kind, muted}` |
| `notification` | новое уведомление | уведомление |
| `user.status` | изменение статуса | `{user_id, online}` |
| `admin.broadcast` | рассылка от админа | `{title, body}` |

## Проверка (Postman)

В коллекции `postman/Jeogram API.postman_collection.json` есть папка
**Realtime (WebSocket / Socket.IO)** с готовыми запросами подключения (`WS /ws`,
`WS /calls/ws`).
