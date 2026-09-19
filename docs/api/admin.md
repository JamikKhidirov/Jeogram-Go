<p align="center">
  <img src="../assets/hdr-admin.svg" width="100%" alt="Админка"/>
</p>
# Админка

Все эндпоинты префикс `/admin` и требуют `Bearer` **и** membership в `ADMIN_USER_IDS`
(в docker-compose по умолчанию `ADMIN_USER_IDS=*` — админом становится любой
авторизованный пользователь; в проде укажите конкретный UUID).

## Пользователи

- `GET /admin/users?limit=50&offset=0` — список пользователей.
- `GET /admin/users/search?q=alice` — поиск.
- `GET /admin/users/{id}` — профиль (email, телефон, IP, статус, роль) + агрегаты:
  число чатов, сообщений, устройств и звонков пользователя.
- `POST /admin/users` — создать аккаунт от лица админа:

```json
{ "email": "user@example.com", "phone": "+7900000000", "password": "secret123", "role": "user" }
```

`role` — `user` (по умолчанию) или `admin`. Возвращает созданный профиль.
Дубликат email/телефона отклоняется с `409`.
- `GET /admin/users/{id}/devices` — устройства пользователя.
- `POST /admin/users/{id}/ban` — заблокировать (`status=banned`).
- `POST /admin/users/{id}/unban` — разблокировать.
- `POST /admin/users/{id}/role` — `{ "role": "admin" | "user" }`.
- `DELETE /admin/users/{id}` — удалить аккаунт.

## Чаты и сообщения

- `GET /admin/chats?limit=50&offset=0`
- `GET /admin/chats/{id}/messages?limit=50&offset=0`
- `GET /admin/messages/search?q=привет`

## Устройства и статистика

- `GET /admin/devices?limit=50&offset=0` — все push-устройства.
- `GET /admin/stats` — агрегаты:

```json
{ "users": 12, "chats": 5, "messages": 340, "calls": 8, "devices": 20 }
```

## Рассылка

`POST /admin/broadcast`

```json
{ "title": "Техработоты", "body": "Сервис будет недоступен" }
```

Отправляет событие `admin.broadcast` всем подключённым пользователям через реалтайм.
