# Аутентификация и подтверждение

Все эндпоинты префикс `/auth`.

## Регистрация

`POST /auth/register`

```json
{
  "email": "alice@example.com",
  "username": "alice",
  "password": "Passw0rd!23",
  "phone": "+79001234567"
}
```

После успешной регистрации на email (или в лог сервера, если SMTP не настроен)
отправляется 6-значный код подтверждения. Токены выдаются сразу, но если включён
флаг `AUTH_REQUIRE_EMAIL_VERIFIED=true` — вход будет запрещён до подтверждения.

Ответ:

```json
{
  "success": true,
  "data": {
    "user": { "id": "uuid", "email": "...", "username": "alice", "display_name": "alice" },
    "access_token": "ey...",
    "refresh_token": "ey..."
  }
}
```

## Вход по email + паролю

`POST /auth/login`

```json
{ "email": "alice@example.com", "password": "Passw0rd!23" }
```

## Подтверждение email кодом

`POST /auth/verify-email` (требует Bearer)

```json
{ "code": "123456" }
```

## Повторная отправка кода

`POST /auth/resend-verification` (требует Bearer) — тело пустое.

## Вход по номеру телефона (OTP)

1. `POST /auth/request-otp`

```json
{ "phone": "+79001234567" }
```

Код «отправляется» на email пользователя (если указан) либо пишется в лог сервера
в dev-режиме.

2. `POST /auth/verify-otp`

```json
{ "phone": "+79001234567", "code": "123456" }
```

Возвращает пару токенов, как при обычном логине.

## Сброс пароля

1. `POST /auth/forgot-password`

```json
{ "email": "alice@example.com" }
```

Всегда возвращает успех (чтобы не раскрывать наличие аккаунта), но при существующем
email отправляет код.

2. `POST /auth/reset-password`

```json
{ "email": "alice@example.com", "code": "123456", "password": "NewPassw0rd!23" }
```

## Обновление токена

`POST /auth/refresh`

```json
{ "refresh_token": "ey..." }
```

## Профиль / выход

- `GET /auth/me` (Bearer) — текущий пользователь.
- `POST /auth/logout` (Bearer) — отзыв refresh-токена (текущее устройство).
- `POST /auth/logout-all` (Bearer) — выход со **всех** устройств: отзывает refresh-токен
  и удаляет все регистрации устройств (`DeviceToken`), эффективно завершая все сессии.
- `GET /auth/sessions` (Bearer) — история активных входов/устройств: список `DeviceToken`
  (платформа, модель, ОС, IP, время входа). Основа функции «история входов».

## Смена учётных данных (требует Bearer)

- `POST /auth/change-password` — `{ "old_password": "...", "new_password": "..." }`.
  После смены все сессии завершаются (`logout-all`).
- `POST /auth/change-email/request` — `{ "new_email": "new@example.com" }`.
  Код подтверждения отправляется на **новый** email (и уведомление на текущий).
- `POST /auth/change-email` — `{ "new_email": "...", "code": "123456" }`.
  Подтверждает смену, проставляет `email_verified=true`.
- `POST /auth/change-phone/request` — `{ "new_phone": "+7900..." }`. OTP уходит
  на email пользователя (или в лог сервера в dev-режиме).
- `POST /auth/change-phone` — `{ "new_phone": "...", "code": "123456" }`.
  Подтверждает смену, проставляет `phone_verified=true`.

## Конфигурация (env)

| Переменная | По умолчанию | Назначение |
| --- | --- | --- |
| `AUTH_REQUIRE_EMAIL_VERIFIED` | `false` | требовать подтверждение email для входа |
| `AUTH_OTP_LENGTH` | `6` | длина OTP/кода |
| `AUTH_CODE_TTL` | `10m` | время жизни кода |
| `SMTP_ENABLED` | `false` | включить отправку email через SMTP |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USER` / `SMTP_PASSWORD` / `SMTP_FROM` | localhost / 587 / "" / "" / no-reply@jeogram.local | параметры SMTP |
