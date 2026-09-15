# Уведомления

Префикс: `/notifications`. Все эндпоинты требуют `Bearer`.

- `POST /notifications/device` — регистрация push-токена устройства:

```json
{ "token": "device-token-abc", "platform": "web", "device_model": "Pixel 7", "os_version": "14" }
```

- `GET /notifications` — лента уведомлений (всех типов: новые сообщения, звонки, системные).
- `POST /notifications/read` — `{ "ids": ["uuid"] }` отметить прочитанными.
- `GET /notifications/unread-count` — число непрочитанных.
- `DELETE /notifications/{id}` — удалить одно уведомление.
- `DELETE /notifications` — удалить все уведомления пользователя.

Уведомления также дублируются в реалтайм через raw WebSocket
(событие `notification`), если клиент подключён.

LiveKit-звонки генерируют in-app уведомления через ту же систему.
