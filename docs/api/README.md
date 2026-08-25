# Документация API Jeogram Messenger

Полная интерактивная спецификация доступна в Swagger UI: `http://localhost:8080/swagger/index.html`
(файл спецификации — `docs/swagger.json`). Этот каталог содержит подробное описание
групп эндпоинтов на русском языке.

## Содержание

- [Аутентификация и подтверждение (auth)](auth.md)
- [Пользователи, профиль, настройки, контакты](users.md)
- [Чаты](chats.md)
- [Сообщения](messages.md)
- [Звонки (WebRTC)](calls.md)
- [Сбор данных с телефона](phone.md)
- [Уведомления](notifications.md)
- [Realtime: WebSocket и Socket.IO](realtime.md)
- [Админка](admin.md)
- [Webhook-уведомления](webhooks.md)
- [Развёртывание (Docker)](deployment.md)

## Базовый URL

```
http://localhost:8080
```

## Общий формат ответа

Все ответы обёрнуты в единый вид:

```json
{
  "success": true,
  "data": { },
  "meta": { "request_id": "..." }
}
```

Ошибки:

```json
{
  "success": false,
  "error": { "code": "unauthorized", "message": "invalid credentials" }
}
```

## Авторизация

Большинство эндпоинтов требуют заголовок:

```
Authorization: Bearer <access_token>
```

Токен выдаётся при `/auth/login`, `/auth/register`, `/auth/verify-otp`.
AccessToken живёт 1 час (настраивается `JWT_ACCESS_TTL`), RefreshToken — 7 суток.

## Быстрый старт (через Postman)

Импортируй сначала окружение `postman/Jeogram.postman_environment.json`, затем коллекцию
`postman/Jeogram API.postman_collection.json`. Папка `Auth → Login` автоматически
сохраняет токен в переменную `{{access_token}}`, которая подставляется во все защищённые
запросы.
