# Звонки (WebRTC)

Префикс: `/calls`. Все эндпоинты требуют `Bearer`.

## Управление звонком

- `POST /calls` — начать звонок:
  `{ "chat_id": "uuid", "type": "video" | "audio", "mode": "peer" | "group" }`.
  `mode` по умолчанию `peer` (1-на-1). `mode=group` переводит звонок в групповой
  режим (SFU-ready) — сигналинг сохраняет пересылку по всем участникам чата,
  готовую к SFU-маршрутизации медиапотоков. Участникам чата через WebSocket
  приходит событие `call.started`.
- `POST /calls/{id}/end` — завершить (только инициатор).
- `GET /calls/{id}` — информация о звонке.
- `GET /calls/history?limit=50&offset=0` — история звонков пользователя.
- `GET /calls/active` — список **активных** (незавершённых) звонков (`status=active`).
- `POST /calls/{id}/join` — присоединиться к групповому звонку (проверяется членство в чате,
  участникам чата рассылается `call.joined`).
- `POST /calls/{id}/mute` — отключить/включить микрофон или камеру во время звонка:
  `{ "kind": "audio" | "video", "muted": true }`. Участникам рассылается
  `call.participant_muted`.
- `POST /calls/{id}/record` — запустить/остановить запись (только инициатор):
  `{ "action": "start" | "stop" }`. Обновляет поле `recording` звонка.

## WebRTC-сигналинг

`GET /calls/ws` — WebSocket-соединение для обмена SDP/ICE между участниками.
Клиент шлёт JSON `{ "type": "offer|answer|ice|call|hangup", "chat_id": "...", "to": "userID", "payload": {...} }`,
сервер пересылает остальным участникам чата (кроме отправителя) как
событие `call.signal`.

## STUN/TURN

`GET /calls/ice-servers` — возвращает массив ICE-серверов для WebRTC:

```json
{
  "ice_servers": [
    { "urls": "stun:stun.l.google.com:19302" },
    { "urls": "turn:turn.example.com:3478", "username": "...", "credential": "..." }
  ],
  "recording_enabled": false
}
```

Конфигурируется env: `RTC_STUN_SERVERS`, `RTC_TURN_SERVERS`, `RTC_TURN_USER`,
`RTC_TURN_PASSWORD`, `RTC_RECORDING_ENABLED`, `RTC_RECORDING_DIR`.

## Запись звонка

`POST /calls/{id}/recording` (только инициатор) — `{ "url": "https://.../rec.webm" }`.
Сохраняет ссылку на запись в поле `recording_url` звонка (доступно через `GET /calls/{id}`).
