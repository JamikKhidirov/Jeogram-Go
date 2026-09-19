<p align="center">
  <img src="../assets/calls.svg" width="100%" alt="Звонки"/>
</p>
# Звонки (LiveKit + WebRTC)

Префикс: `/calls`. Все эндпоинты требуют `Bearer`.

## LiveKit (аудио/видео звонки с записью)

### Создание звонка

`POST /calls/livekit/start` — создать LiveKit комнату и начать звонок:
`{ "chat_id": "uuid", "type": "audio" | "video", "group": false }`.
Возвращает `{ call, room_name }`. При `group: true` — групповой режим до 50 участников.

### Управление

- `POST /calls/livekit/{id}/end` — завершить звонок (только инициатор).
- `POST /calls/livekit/{id}/join` — присоединиться к групповому звонку, получить `token` и `room_name`.
- `GET /calls/livekit/{id}/token` — получить JWT-токен для входа в LiveKit комнату.
- `GET /calls/livekit/{id}/participants` — список участников LiveKit комнаты.
- `POST /calls/livekit/{id}/mute` — мьют микрофона/камеры: `{ "kind": "audio" | "video", "muted": true }`.

### Запись

- `POST /calls/livekit/{id}/record/start` — запустить серверную запись комнаты.
- `POST /calls/livekit/{id}/record/stop` — остановить запись, получить `recording_url`.
- `GET /calls/livekit/{id}/recording` — получить ссылку на запись.

### Комнаты и ICE

- `GET /calls/livekit/rooms` — список активных LiveKit комнат.
- `GET /calls/livekit/ice-servers` — STUN/TURN серверы для WebRTC.

## WebRTC-сигналинг (опционально)

`WS /calls/ws` — WebSocket-соединение для обмена SDP/ICE между участниками.
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

## Запись звонка (WebRTC, без LiveKit)

`POST /calls/{id}/record` (только инициатор) — `{ "action": "start" | "stop" }`.
Обновляет поле `recording` звонка.
`POST /calls/{id}/recording` — `{ "url": "https://.../rec.webm" }`.
Сохраняет ссылку на запись в поле `recording_url` звонка (доступно через `GET /calls/{id}`).
