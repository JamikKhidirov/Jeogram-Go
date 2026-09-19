<p align="center">
  <img src="../assets/hdr-chat.svg" width="100%" alt="Пользователи, профиль, контакты"/>
</p>
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

- `GET /user/online` — список **онлайн-друзей** (контактов со статусом `online`;
  пользователи со статусом `invisible` скрываются, оффлайн-контакты не возвращаются).
- `POST /user/status` — установить статус присутствия: `{ "status": "online" | "dnd" | "invisible" }`.
  `dnd` — «не беспокоить», `invisible` — «невидимка» (скрывает из `/user/online` других).
  Статус хранится в WebSocket-хабе; без активного соединения пользователь считается `offline`.

## Экспорт данных (GDPR)

`GET /user/export` — выгрузка всех данных пользователя.

## Мои устройства

- `GET /user/devices` — список активных устройств (платформа, модель, ОС, IP,
  время регистрации/обновления). Основано на записях токенов уведомлений
  (`notification_device_tokens`).
- `DELETE /user/devices/{platform}` — удалить регистрацию устройства
  (удалённый выход с устройства). `platform` — `ios` / `android` / `web`.

## Удаление аккаунта

`DELETE /user/account` — soft-delete аккаунта.

## Контакты

Префикс `/contacts`.

- `GET /contacts` — мои контакты.
- `POST /contacts` — `{ "user_id": "uuid" }` (добавить/запросить).
- `GET /contacts/requests` — входящие заявки.
- `POST /contacts/{user_id}/accept` — принять заявку.
- `DELETE /contacts/{user_id}` — удалить контакт.
- `POST /contacts/sync` — массовая синхронизация с телефонной книгой:
  `{ "user_ids": ["uuid1", "uuid2", ...] }`. Добавляет только реально существующих
  пользователей, которых ещё нет в контактах; возвращает `{ "added": N }`.
- `POST /contacts/{user_id}/block` — заблокировать пользователя (привязка к чёрному
  списку аккаунта `user_blocklist`).
