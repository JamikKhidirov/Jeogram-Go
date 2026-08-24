# Пользователи, профиль, настройки, контакты

Префикс: `/user`. Все эндпоинты требуют `Bearer`.

## Профиль

- `GET /user/profile` — текущий профиль.
- `PUT /user/profile`

```json
{ "display_name": "Алиса", "avatar_url": "https://...", "bio": "привет" }
```

## Настройки

- `GET /user/settings`
- `PUT /user/settings`

```json
{ "theme": "dark", "notifications_enabled": true, "language": "ru" }
```

## Поиск пользователей

`GET /user/search?q=alice`

## Блокировки

- `POST /user/block` — `{ "user_id": "uuid" }`
- `GET /user/blocks` — список заблокированных.
- `DELETE /user/block/{user_id}` — разблокировать.

## Присутствие (онлайн)

`GET /user/presence?ids=u1,u2` — статусы онлайн/оффлайн (используется WebSocket-хабом).

## Экспорт данных (GDPR)

`GET /user/export` — выгрузка всех данных пользователя.

## Удаление аккаунта

`DELETE /user/account` — soft-delete аккаунта.

## Контакты

Префикс `/contacts`.

- `GET /contacts` — мои контакты.
- `POST /contacts` — `{ "user_id": "uuid" }` (добавить/запросить).
- `GET /contacts/requests` — входящие заявки.
- `POST /contacts/{user_id}/accept` — принять заявку.
- `DELETE /contacts/{user_id}` — удалить контакт.
