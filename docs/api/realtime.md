# Realtime: WebSocket и Socket.IO

Сервер поддерживает **два** транспорта для одних и тех же событий. Функционально
они эквивалентны — используйте то, что удобнее для клиента.

## WebSocket (`/ws`)

Подключение (raw WebSocket):

```
ws://localhost:8080/ws?token=<access_token>
```

Аутентификация: заголовок `Authorization: Bearer <token>` либо query-параметр `?token=`.
После подключения сервер шлёт Ping каждые 30с. Клиент может просто держать
соединение — это обеспечивает синхронизацию между всеми устройствами пользователя
(один пользователь может быть подключён с телефона и десктопа одновременно).

Входящее событие (JSON):

```json
{ "type": "<event>", "payload": { ... } }
```

## Socket.IO (`/socket.io/`)

```
http://localhost:8080/socket.io/?token=<access_token>&EIO=4&transport=websocket
```

События идентичны WebSocket. Каждый пользователь подключается в приватную
комнату `u:<userID>`.

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
| `notification` | новое уведомление | уведомление |
| `user.status` | изменение статуса | `{user_id, online}` |
| `admin.broadcast` | рассылка от админа | `{title, body}` |

## Проверка (Postman)

В коллекции `postman/Jeogram.postman_collection.json` есть папка
**Realtime (WebSocket)** с готовым запросом подключения.
