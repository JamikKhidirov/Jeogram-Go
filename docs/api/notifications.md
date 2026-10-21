# Уведомления

Префикс: `/notifications`. Все эндпоинты требуют `Bearer`.

## Регистрация устройства (push-токен)

`POST /notifications/device` — зарегистрировать push-токен устройства:

```json
{
  "token": "<device_push_token>",
  "platform": "ios|android|web",
  "device_model": "iPhone 14",
  "os_version": "17.0",
  "app_version": "1.0.0",
  "locale": "ru-RU",
  "timezone": "Europe/Moscow"
}
```

| Поле | Описание |
|------|----------|
| `token` | Push-токен: Android — FCM registration token, iOS — APNs device token |
| `platform` | `ios`, `android` или `web` |
| `device_model` | Модель устройства |
| `os_version` | Версия ОС |
| `app_version` | Версия приложения |
| `locale` | Язык/регион |
| `timezone` | Часовой пояс |

Сервер сохраняет токен в таблицу `device_tokens` и автоматически дописывает IP и User-Agent из запроса.

## Лента уведомлений

- `GET /notifications` — лента уведомлений (всех типов: новые сообщения, звонки, системные).
- `GET /notifications/unread-count` — число непрочитанных.
- `POST /notifications/read` — `{ "ids": ["uuid"] }` отметить прочитанными.
- `DELETE /notifications/{id}` — удалить одно уведомление.
- `DELETE /notifications` — удалить все уведомления пользователя.

## Типы уведомлений

| Тип | Когда приходит | Push отправляется |
|-----|---------------|-------------------|
| `message.new` | Новое сообщение в чате | ✅ Android (FCM) + iOS (APNs) |
| `message.edited` | Сообщение отредактировано | ❌ Только in-app |
| `message.read` | Сообщение прочитано | ❌ Только in-app |
| `call.incoming` | Входящий аудио/видео звонок | ✅ Android (FCM) + iOS (APNs) |
| `call.ended` | Звонок завершён | ❌ Только in-app |
| `call.participant_muted` | Участник отключил микрофон | ❌ Только in-app |
| `system.alert` | Системное уведомление от админа | ✅ Android (FCM) + iOS (APNs) |
| `auth.code_sent` | Код подтверждения на email | ❌ Только in-app |
| `profile.updated` | Профиль обновлён | ❌ Только in-app |

## Push-уведомления Android (FCM)

Настройка в `.env`:

```
PUSH_ENABLED=true
FCM_SERVER_KEY=<ваш_серверный_ключ_из_Firebase_Console>
# или
FCM_OAUTH_TOKEN=<OAuth_токен>
```

Получить ключ: [Firebase Console](https://console.firebase.google.com) → Project Settings → Cloud Messaging → Server key.

## Push-уведомления iOS (APNs)

Настройка в `.env`:

```
PUSH_ENABLED=true
APNS_KEY_ID=<ключ_из_Apple_Developer>
APNS_TEAM_ID=<команда_Apple>
APNS_KEY_PATH=/путь/к/authkey.p8
APNS_BUNDLE_ID=<bundle_идентификатор>
APNS_PRODUCTION=false  # true для продакшена
```

Получить ключ: [Apple Developer](https://developer.apple.com) → Certificates, Identifiers & Profiles → Keys → APNs.

## Push при звонке и сообщении

При создании звонка (`POST /calls/livekit/start` или `POST /calls`) сервер:

1. Создаёт запись звонка в PostgreSQL
2. Создаёт LiveKit комнату
3. Рассылает WebSocket событие `call.started` всем участникам чата
4. Отправляет push-уведомление `call.incoming` всем устройствам участников

При новом сообщении (`POST /messages` / `POST /chats/{id}/messages`) сервер:

1. Сохраняет сообщение в PostgreSQL
2. Кэширует в Redis (TTL 60с)
3. Рассылает WebSocket событие `message.new` участникам чата
4. Отправляет push-уведомление `message.new` устройствам получателей

## Админ-рассылка

`POST /admin/broadcast` — отправить push всем онлайн-пользователям:

```json
{
  "title": "Заголовок",
  "body": "Текст уведомления"
}
```

Сервер рассылает через WebSocket (`admin.broadcast`) и push (FCM/APNs).

## Отключение push

Если `PUSH_ENABLED=false` (по умолчанию), push-уведомления только логируются, но не отправляются. Это позволяет тестировать весь поток без реальных ключей FCM/APNs.
