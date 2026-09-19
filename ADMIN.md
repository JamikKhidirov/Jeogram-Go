<p align="center">
  <img src="docs/assets/hdr-admin.svg" width="100%" alt="Админка и телеметрия"/>
</p>
# Админка и телеметрия устройств (Android / iOS)

## Назначение админа

Админ-доступ даётся по списку ID пользователей в переменной окружения
`ADMIN_USER_IDS` (через запятую). Все админ-роуты защищены middleware
`RequireAdmin`, который пропускает только перечисленных пользователей,
иначе возвращает `403 forbidden`.

```bash
# .env — только эти пользователи получают админ-доступ
ADMIN_USER_IDS=11111111-1111-1111-1111-111111111111,22222222-2222-2222-2222-222222222222
```

Для личного/self-hosted использования можно временно открыть админ-доступ
всем авторизованным пользователям (не для продакшена!):

```bash
ADMIN_USER_IDS=*
```

Свой ID пользователь узнаёт через `GET /auth/me` или `GET /user/me`
(поле `data.id`) и подставляет в `ADMIN_USER_IDS`, после чего перезапускает
приложение.

## Что клиент (Android / iOS) может отправить о себе

При регистрации push-токена (`POST /notifications/device`) клиент передаёт
дополнительную телеметрию устройства. Сервер **сам** дописывает IP
(из `X-Forwarded-For` / `RemoteAddr`) и `User-Agent` (из заголовка запроса).

| Поле запроса      | Откуда берётся у клиента (Android / iOS)                                                                 |
|-------------------|----------------------------------------------------------------------------------------------------------|
| `platform`        | `ios` / `android` / `web` (выбирается клиентом)                                                          |
| `token`           | Push-токен: Android — FCM `token`, iOS — APNs device token                                               |
| `device_model`    | Android: `Build.MODEL` / `Build.MANUFACTURER`; iOS: `UIDevice.current.model` + `utsname.machine`         |
| `os_version`      | Android: `Build.VERSION.RELEASE` (`SDK Int`); iOS: `UIDevice.current.systemVersion`                      |
| `app_version`     | Android: `PackageInfo.versionName`; iOS: `CFBundleShortVersionString` + `CFBundleVersion`                |
| `locale`          | Android: `Locale.getDefault()`; iOS: `Locale.current.languageCode`                                        |
| `timezone`        | Android: `TimeZone.getDefault().id`; iOS: `TimeZone.current.identifier`                                  |
| `last_ip`         | **сервер**: `X-Forwarded-For` → `RemoteAddr` (надёжно при прокси/Nginx/Load Balancer)                    |
| `user_agent`      | **сервер**: заголовок `User-Agent` (OkHttp/UA iOS и т.п.)                                                |

Сервер сохраняет всё это в таблицу `device_tokens` (одна запись на
`user_id`+`platform`, обновляется при каждом заходе). Поле `last_ip`
дублируется и в профиль пользователя (`users.last_seen_ip`, `user_agent`,
`last_seen_at`) при логине/регистрации устройства.

## Админ-эндпоинты

Все требуют `Authorization: Bearer <access_token>` админа.

| Метод | Путь | Описание |
|--------|------|----------|
| GET | `/admin/users` | Список пользователей (с `last_seen_ip`, `user_agent`) |
| GET | `/admin/users/{id}` | Полная информация о пользователе |
| GET | `/admin/users/{id}/devices` | Устройства пользователя (модель, ОС, app, локаль, IP, UA) |
| GET | `/admin/chats` | Все чаты системы |
| GET | `/admin/chats/{id}/messages` | Сообщения конкретного чата |
| GET | `/admin/messages/search?q=` | Глобальный поиск сообщений по тексту |
| GET | `/admin/devices` | Все устройства (IP, модель, ОС, локаль) |
| GET | `/admin/stats` | Агрегированная статистика (users/chats/messages/calls/devices) |

Пример (после того как свой ID добавлен в `ADMIN_USER_IDS`):

```bash
curl http://localhost:8080/admin/devices -H "Authorization: Bearer <admin_token>"
curl "http://localhost:8080/admin/messages/search?q=привет" -H "Authorization: Bearer <admin_token>"
curl http://localhost:8080/admin/stats -H "Authorization: Bearer <admin_token>"
```

## Реалтайм (WebSocket)

Единственный транспорт событий — **raw WebSocket**:

- `WS /ws` (Bearer в заголовке или `?token=`) — удобно тестить прямо в Postman.
- Socket.IO из проекта удалён (см. [SOCKETIO.md](SOCKETIO.md)).

События: `message.new`, `message.read`, `typing`, `presence`, `notification`,
`call.signal`, `call.started`, `call.ended`. Подробнее: [WEBSOCKET.md](WEBSOCKET.md),
[docs/api/realtime.md](docs/api/realtime.md).
