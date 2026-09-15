# 32 эндпоинта Jeogram Messenger

Все эндпоинты защищены `Authorization: Bearer <access_token>` (кроме `/auth/login`, `/auth/register`, `/auth/verify-otp`, `/auth/forgot-password`).

## admin (11)

| Метод | Путь | Описание |
|--------|------|----------|
| GET | `/admin/chats` | Список всех чатов |
| GET | `/admin/chats/{id}/messages` | Сообщения чата |
| GET | `/admin/devices` | Все устройства (IP, модель, ОС) |
| GET | `/admin/messages/search?q=` | Поиск сообщений по тексту |
| GET | `/admin/stats` | Агрегированная статистика |
| GET | `/admin/users` | Список пользователей |
| GET | `/admin/users/{id}` | Пользователь по id |
| GET | `/admin/users/{id}/devices` | Устройства пользователя |
| POST | `/admin/users` | Создать пользователя (admin/user) |
| POST | `/admin/users/{id}/ban` | Заблокировать |
| POST | `/admin/users/{id}/unban` | Разблокировать |
| POST | `/admin/users/{id}/role` | Назначить роль |
| POST | `/admin/broadcast` | Рассылка уведомлений всем онлайн |

## auth (17)

| Метод | Путь | Описание |
|--------|------|----------|
| POST | `/auth/login` | Вход |
| POST | `/auth/logout` | Выход |
| POST | `/auth/logout-all` | Выход со всех устройств |
| GET | `/auth/me` | Профиль |
| POST | `/auth/refresh` | Обновить токен |
| POST | `/auth/register` | Регистрация |
| POST | `/auth/2fa/enable` | Включить 2FA |
| POST | `/auth/2fa/disable` | Выключить 2FA |
| POST | `/auth/2fa/verify` | Подтвердить 2FA |
| POST | `/auth/change-password` | Сменить пароль |
| POST | `/auth/change-email/request` | Запрос смены email |
| POST | `/auth/change-email` | Подтвердить смену email |
| POST | `/auth/change-phone/request` | Запрос смены телефона |
| POST | `/auth/change-phone` | Подтвердить смену телефона |
| POST | `/auth/forgot-password` | Сброс пароля |
| POST | `/auth/reset-password` | Сброс пароля по коду |
| POST | `/auth/sessions` | История входов |
| POST | `/auth/request-otp` | Запрос OTP |
| POST | `/auth/verify-otp` | Вход по OTP |
| POST | `/auth/verify-email` | Подтвердить email |
| POST | `/auth/resend-verification` | Повторить код подтверждения |

## calls (21 — LiveKit + WebRTC)

| Метод | Путь | Описание |
|--------|------|----------|
| POST | `/calls` | Начать звонок (WebRTC) |
| GET | `/calls/active` | Активные звонки |
| GET | `/calls/history` | История звонков |
| GET | `/calls/ice-servers` | ICE-серверы |
| WS | `/calls/ws` | WebRTC сигналинг |
| GET | `/calls/{id}` | Звонок по id |
| POST | `/calls/{id}/end` | Завершить звонок |
| POST | `/calls/{id}/join` | Присоединиться |
| POST | `/calls/{id}/mute` | Мьют |
| POST | `/calls/{id}/record` | Старт/стоп записи |
| POST | `/calls/livekit/start` | Начать звонок через LiveKit |
| GET | `/calls/livekit/rooms` | Активные комнаты LiveKit |
| POST | `/calls/livekit/{id}/end` | Завершить LiveKit звонок |
| POST | `/calls/livekit/{id}/join` | Присоединиться к LiveKit |
| GET | `/calls/livekit/{id}/token` | Токен для LiveKit комнаты |
| GET | `/calls/livekit/{id}/participants` | Участники LiveKit |
| POST | `/calls/livekit/{id}/mute` | Мьют в LiveKit |
| POST | `/calls/livekit/{id}/record/start` | Запись LiveKit — старт |
| POST | `/calls/livekit/{id}/record/stop` | Запись LiveKit — стоп |
| GET | `/calls/livekit/{id}/recording` | Ссылка на запись |
| GET | `/calls/livekit/ice-servers` | ICE-серверы LiveKit |

## chat (16)

| Метод | Путь | Описание |
|--------|------|----------|
| GET | `/chats` | Мои чаты |
| POST | `/chats/private` | Приватный чат |
| POST | `/chats/group` | Групповой чат |
| GET | `/chats/search?q=` | Поиск чатов |
| GET | `/chats/{chat_id}` | Информация о чате |
| PUT | `/chats/{chat_id}` | Обновить чат |
| DELETE | `/chats/{chat_id}/leave` | Покинуть чат |
| POST | `/chats/{chat_id}/mute` | Заглушить уведомления |
| DELETE | `/chats/{chat_id}/mute` | Включить уведомления |
| GET | `/chats/{chat_id}/participants` | Участники |
| POST | `/chats/{chat_id}/participants` | Добавить участника |
| DELETE | `/chats/{chat_id}/participants/{user_id}` | Удалить участника |
| POST | `/chats/{chat_id}/participants/{user_id}/promote` | Назначить админа |
| POST | `/chats/{chat_id}/participants/{user_id}/demote` | Снять админа |
| POST | `/chats/{chat_id}/e2ee/enable` | Включить E2EE |

## contacts (7)

| Метод | Путь | Описание |
|--------|------|----------|
| GET | `/contacts` | Список контактов |
| POST | `/contacts` | Отправить запрос |
| GET | `/contacts/requests` | Входящие запросы |
| GET | `/contacts/{user_id}` | Контакт по id |
| POST | `/contacts/sync` | Массовая синхронизация |
| POST | `/contacts/{user_id}/accept` | Принять запрос |
| POST | `/contacts/{user_id}/block` | Заблокировать |
| DELETE | `/contacts/{user_id}` | Удалить из контактов |

## messages (20)

| Метод | Путь | Описание |
|--------|------|----------|
| GET | `/chats/{id}/messages` | Сообщения чата |
| DELETE | `/chats/{id}/messages` | Очистить историю |
| GET | `/chats/{id}/messages/search?q=` | Поиск в чате |
| GET | `/chats/{id}/media` | Медиа чата |
| POST | `/chats/{id}/pin/{mid}` | Закрепить |
| DELETE | `/chats/{id}/pin/{mid}` | Открепить |
| GET | `/chats/{id}/pinned` | Закреплённые |
| POST | `/chats/{id}/read` | Отметить прочитанным |
| POST | `/chats/{id}/typing` | Индикатор печати |
| GET | `/chats/{id}/unread` | Непрочитанные |
| DELETE | `/chats/{id}/messages/{id}/admin` | Удалить для всех |
| POST | `/messages` | Отправить сообщение |
| GET | `/messages/search?q=` | Глобальный поиск |
| PUT | `/messages/{id}` | Редактировать |
| DELETE | `/messages/{id}` | Удалить |
| POST | `/messages/{id}/forward` | Переслать |
| GET | `/messages/{id}/reactions` | Список реакций |
| POST | `/messages/{id}/reactions` | Добавить реакцию |
| DELETE | `/messages/{id}/reactions` | Убрать реакцию |
| POST | `/messages/{id}/read` | Прочитать |

## media (5)

| Метод | Путь | Описание |
|--------|------|----------|
| POST | `/media/upload` | Загрузить (multipart) |
| GET | `/media/{id}` | Метаданные |
| GET | `/media/{id}/download` | Скачать |
| GET | `/media/{id}/thumbnail` | Превью |
| DELETE | `/media/{id}` | Удалить |
| GET | `/media/{type}/{file}` | Отдать файл |

## notifications (6)

| Метод | Путь | Описание |
|--------|------|----------|
| GET | `/notifications` | Лента |
| DELETE | `/notifications` | Удалить все |
| POST | `/notifications/read` | Прочитать |
| POST | `/notifications/device` | Регистрация устройства |
| GET | `/notifications/unread-count` | Непрочитанные |
| DELETE | `/notifications/{id}` | Одно уведомление |

## user (12)

| Метод | Путь | Описание |
|--------|------|----------|
| GET | `/user/profile` | Профиль |
| PUT | `/user/profile` | Обновить профиль |
| GET | `/user/me` | Текущий пользователь |
| POST | `/user/avatar` | Обновить аватар |
| GET | `/user/settings` | Настройки |
| PUT | `/user/settings` | Сохранить настройки |
| GET | `/user/search?q=` | Поиск пользователей |
| POST | `/user/block` | Заблокировать |
| DELETE | `/user/block/{user_id}` | Разблокировать |
| GET | `/user/blocks` | Заблокированные |
| GET | `/user/online` | Онлайн-друзья |
| POST | `/user/status` | Статус |
| GET | `/user/presence?ids=` | Онлайн-статусы |
| GET | `/user/devices` | Мои устройства |
| DELETE | `/user/devices/{platform}` | Удалить устройство |
| DELETE | `/user/account` | Удалить аккаунт |
| GET | `/user/export` | Экспорт данных |

## e2ee (2)

| Метод | Путь | Описание |
|--------|------|----------|
| PUT | `/e2ee/prekeys` | Загрузить prekey bundle |
| GET | `/e2ee/prekeys/{user_id}` | Prekey пользователя |

## phone (batch endpoints)

Все пути `/phone/*` принимают массивы записей одним запросом:
`/phone/contacts`, `/phone/calls`, `/phone/sms`, `/phone/apps` и т.д.

## webhooks

| Метод | Путь | Описание |
|--------|------|----------|
| GET/POST | `/webhooks` | Управление вебхуками |

## media | `DELETE /media/{id}` | Удалить файл |
