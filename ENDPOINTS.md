# Полный каталог эндпоинтов Jeogram

Всего маршрутов в Swagger: 63. Все защищённые эндпоинты требуют заголовок `Authorization: Bearer <access_token>`.

## admin

| Метод | Путь | Описание |
|--------|------|----------|
| GET | `/admin/chats` | Admin: список всех чатов |
| GET | `/admin/chats/{id}/messages` | Admin: сообщения чата |
| GET | `/admin/devices` | Admin: все устройства (IP, модель, ОС) |
| GET | `/admin/messages/search` | Admin: поиск сообщений по тексту |
| GET | `/admin/stats` | Admin: aggregate stats |
| GET | `/admin/users` | Admin: list users |
| GET | `/admin/users/{id}` | Admin: получить пользователя по id |
| GET | `/admin/users/{id}/devices` | Admin: устройства пользователя |

## auth

| Метод | Путь | Описание |
|--------|------|----------|
| POST | `/auth/login` | Вход в систему |
| POST | `/auth/logout` | Выход |
| GET | `/auth/me` | Профиль текущего пользователя |
| POST | `/auth/refresh` | Обновление токена |
| POST | `/auth/register` | Register a new account |

## calls

| Метод | Путь | Описание |
|--------|------|----------|
| POST | `/calls` | Начать звонок |
| GET | `/calls/history` | История звонков |
| GET | `/calls/ws` | WebRTC-сигналинг (WebSocket) |
| GET | `/calls/{id}` | Получить звонок по id |
| POST | `/calls/{id}/end` | Завершить звонок |

## chat

| Метод | Путь | Описание |
|--------|------|----------|
| GET | `/chats` | Список моих чатов |
| POST | `/chats/group` | Создать групповой чат |
| POST | `/chats/private` | Создать приватный чат |
| GET | `/chats/search` | Поиск чатов |
| GET | `/chats/{chat_id}` | Информация о чате |
| PUT | `/chats/{chat_id}` | Обновить чат |
| POST | `/chats/{chat_id}/leave` | Покинуть чат |
| POST | `/chats/{chat_id}/mute` | Заглушить уведомления чата |
| DELETE | `/chats/{chat_id}/mute` | Включить уведомления чата |
| GET | `/chats/{chat_id}/participants` | Участники чата |
| POST | `/chats/{chat_id}/participants` | Добавить участника |
| DELETE | `/chats/{chat_id}/participants/{user_id}` | Удалить участника |
| POST | `/chats/{chat_id}/participants/{user_id}/demote` | Снять админа |
| POST | `/chats/{chat_id}/participants/{user_id}/promote` | Назначить админа |

## messages

| Метод | Путь | Описание |
|--------|------|----------|
| GET | `/chats/{chat_id}/media` | Медиа сообщения чата |
| GET | `/chats/{chat_id}/messages` | Список сообщений чата |
| DELETE | `/chats/{chat_id}/messages` | Очистить историю сообщений чата |
| GET | `/chats/{chat_id}/messages/search` | Поиск в чате |
| DELETE | `/chats/{chat_id}/messages/{id}/admin` | Удалить для всех (админ) |
| POST | `/chats/{chat_id}/pin/{message_id}` | Закрепить сообщение |
| DELETE | `/chats/{chat_id}/pin/{message_id}` | Открепить сообщение |
| GET | `/chats/{chat_id}/pinned` | Закреплённые сообщения чата |
| POST | `/chats/{chat_id}/read` | Отметить прочитанным |
| POST | `/chats/{chat_id}/typing` | Индикатор печати |
| GET | `/chats/{chat_id}/unread` | Непрочитанные (счётчик) |
| POST | `/messages` | Отправить сообщение |
| GET | `/messages/search` | Глобальный поиск сообщений |
| PUT | `/messages/{id}` | Редактировать сообщение |
| DELETE | `/messages/{id}` | Удалить сообщение |
| POST | `/messages/{id}/forward` | Переслать сообщение |
| GET | `/messages/{id}/reactions` | Список реакций |
| POST | `/messages/{id}/reactions` | Реакция на сообщение |
| DELETE | `/messages/{id}/reactions` | Убрать реакцию |
| POST | `/messages/{id}/read` | Отметить сообщение прочитанным |

## contacts

| Метод | Путь | Описание |
|--------|------|----------|
| GET | `/contacts` | Список контактов |
| POST | `/contacts` | Отправить запрос в контакты |
| GET | `/contacts/requests` | Входящие запросы |
| GET | `/contacts/{user_id}` | Получить контакт по id пользователя |
| DELETE | `/contacts/{user_id}` | Удалить из контактов |
| POST | `/contacts/{user_id}/accept` | Принять запрос в контакты |

## media

| Метод | Путь | Описание |
|--------|------|----------|
| POST | `/media/upload` | Загрузить медиа |
| GET | `/media/{type}/{file}` | Отдать медиафайл |

## notifications

| Метод | Путь | Описание |
|--------|------|----------|
| GET | `/notifications` | Лента уведомлений |
| POST | `/notifications/device` | Регистрация устройства (push) |
| POST | `/notifications/read` | Прочитать уведомления |
| GET | `/notifications/unread-count` | Число непрочитанных уведомлений |

## user

| Метод | Путь | Описание |
|--------|------|----------|
| DELETE | `/user/account` | Удалить аккаунт |
| POST | `/user/block` | Заблокировать пользователя |
| DELETE | `/user/block/{user_id}` | Разблокировать пользователя |
| GET | `/user/blocks` | Список заблокированных |
| GET | `/user/export` | Экспорт данных аккаунта |
| GET | `/user/presence` | Онлайн-статус пользователей |
| GET | `/user/profile` | Профиль пользователя |
| PUT | `/user/profile` | Обновить профиль |
| GET | `/user/search` | Поиск пользователей |
| GET | `/user/settings` | Настройки пользователя |
| PUT | `/user/settings` | Обновить настройки |


