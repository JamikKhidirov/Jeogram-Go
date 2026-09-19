<p align="center">
  <img src="../assets/messenger.svg" width="100%" alt="Сообщения и медиа"/>
</p>
# Сообщения

Префикс: `/messages` и `/chats/{chat_id}/messages`. Все эндпоинты требуют `Bearer`.

## Отправка и чтение

- `POST /messages`

```json
{ "chat_id": "uuid", "type": "text", "text": "Привет!", "reply_to": "uuid" }
```

Поддерживаемые `type`: `text`, `voice`, `image`, `video`, `document`.
Для `voice`/`image`/`video`/`document` обязателен `media_url`.
После отправки всем участникам чата через WebSocket приходит событие `message.new`.

### Отложенная отправка (scheduled)

В `POST /messages` можно передать `scheduled_at` (RFC3339, в будущем):

```json
{ "chat_id": "uuid", "type": "text", "text": "Напоминание", "scheduled_at": "2026-01-01T12:00:00Z" }
```

Сообщение сохраняется со `status: scheduled` и **не доставляется** сразу. Фоновый
воркер сервера (ticker 5с) публикует наступившие сообщения, переводя их в `sent` и
рассылая `message.new`. Поле `status` (`sent`/`scheduled`) и `scheduled_at`
возвращаются в `PublicMessage`.

- `GET /chats/{chat_id}/messages?limit=50&offset=0` — история (пагинация).
- `GET /messages/{id}` — одно сообщение.
- `PUT /messages/{id}` — редактирование (в пределах окна `MESSAGE_EDIT_WINDOW`):

```json
{ "text": "Изменено" }
```

Возвращает сообщение с `edit_version` и `edited_at`. Всем устройствам уходит
событие `message.edited`.

## Удаление

- `DELETE /messages/{id}` — удалить у себя (soft-delete для текущего пользователя).
- `DELETE /chats/{chat_id}/messages/{id}/admin` — удалить **для всех** (только админ/владелец
  чата, в пределах окна `MESSAGE_DELETE_FOR_ALL_WINDOW`). Рассылается событие
  `message.deleted_for_all`.

## Реакции

- `POST /messages/{id}/reactions` — `{ "emoji": "👍" }`.
- `DELETE /messages/{id}/reactions`.
- `GET /messages/{id}/reactions`.

## Медиа-файлы

Префикс `/media`. Все эндпоинты требуют `Bearer` (кроме публичной раздачи по
`/media/{type}/{file}`, если storage публичный).

- `POST /media/upload` — загрузка файла (multipart `file` + `type`:
  `image`/`voice`/`video`/`document`). Возвращает `{ "id": "uuid", "url": "/media/{type}/{file}" }`.
  Поддерживаются изображения, голосовые, **видео** и **документы** (любые типы из
  `MEDIA_ALLOWED_TYPES`/`MEDIA_ALLOWED_EXT`). Имя файла генерируется как UUID, чтобы
  исключить коллизии и path-traversal.
- `GET /media/{id}` — метаданные загруженного файла (по записи `media`).
- `GET /media/{id}/download` — скачивание файла (устанавливает `Content-Disposition: attachment`).
- `GET /media/{id}/thumbnail` — отдача миниатюры (для изображений — уменьшенная копия;
  для остальных типов — сам файл/заглушка).
- `DELETE /media/{id}` — удаление файла и записи `media`.
- `GET /media/{type}/{file}` — прямая отдача файла (обратная совместимость).

Конфигурация env: `MEDIA_DIR`, `MEDIA_MAX_SIZE`, `MEDIA_ALLOWED_TYPES`, `MEDIA_ALLOWED_EXT`.

## Пересылка и поиск

- `POST /messages/{id}/forward` — `{ "chat_id": "uuid" }`.
- `GET /chats/{chat_id}/messages/search?q=привет` — поиск по чату.
- `GET /messages/search?q=привет` — глобальный поиск по своим чатам.

## Прочитанное

- `POST /messages/{id}/read` — отметить одно сообщение прочитанным (receipt).
- `POST /chats/{chat_id}/read` — отметить пачку/весь чат прочитанным.

## Конфигурация (env)

| Переменная | По умолчанию | Назначение |
| --- | --- | --- |
| `MESSAGE_EDIT_WINDOW` | `24h` | окно редактирования сообщения |
| `MESSAGE_DELETE_FOR_ALL_WINDOW` | `24h` | окно удаления «для всех» |

## Кэширование истории сообщений

`GET /chats/{chat_id}/messages` кэшируется в Redis под ключом
`msgs:chat:{chatID}:{limit}:{offset}` (TTL 30с). Кэш сбрасывается при отправке,
редактировании, удалении, пересылке, очистке истории и закрепе/открепе сообщений.
При `REDIS_ENABLED=false` кэш прозрачно отключается.
