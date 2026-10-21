#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Генератор Postman-коллекции для Jeogram (мессенджер на Go).

Результат:
  postman/Jeogram API.postman_collection.json  - все HTTP + WebSocket + Socket.IO + LiveKit
  postman/Jeogram.postman_environment.json     - переменные окружения

Как пользоваться:
  1. Импортировать ОБА файла в Postman (сначала Environment, потом Collection).
  2. В правом верхнем углу выбрать окружение "Jeogram Local".
  3. Открыть папку "01. Auth" -> нажать "Регистрация" (или "Вход") ->
     токены автоматически подставятся во все остальные запросы.
  4. Папки пронумерованы по порядку работы: auth -> users -> contacts -> chats ->
     messages -> media -> calls -> realtime -> phone -> admin -> cleanup.

Источник истины по HTTP-эндпоинтам - docs/swagger.json (Swagger 2.0),
регенерируется командой `make swagger`. Описания, тела запросов и автозахват
идентификаторов заданы вручную в CURATED, остальные эндпоинты
(телеметрия телефона, админка) генерируются из swagger автоматически.
"""
import json
import os

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SWAGGER = os.path.join(ROOT, "docs", "swagger.json")
OUT_COLLECTION = os.path.join(ROOT, "postman", "Jeogram API.postman_collection.json")
OUT_ENV = os.path.join(ROOT, "postman", "Jeogram.postman_environment.json")

SCHEMA = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"

# --------------------------------------------------------------------------
# Папки коллекции
# --------------------------------------------------------------------------
FOLDERS = [
    ("start", "00. Старт и диагностика",
     "Проверка живости сервиса и документация. Начните с /health — если он отвечает, "
     "бэкенд поднят. Swagger UI открывается на /swagger/index.html."),
    ("auth", "01. Auth — регистрация и вход",
     "Создание аккаунта и получение JWT. Запрос «Регистрация» или «Вход» автоматически "
     "кладёт access_token/refresh_token в переменные коллекции — все остальные папки "
     "используют их автоматически (Authorization: Bearer {{access_token}})."),
    ("auth_sec", "02. Auth — сессии, пароль, 2FA",
     "Управление сессиями, смена пароля, двухфакторная аутентификация, смена email/телефона."),
    ("users", "03. Users — профиль и настройки",
     "Профиль текущего пользователя, настройки темы/языка, поиск людей, блокировки, "
     "присутствие (presence) и онлайн-список."),
    ("contacts", "04. Contacts — контакты",
     "Адресная книга: добавление, принятие заявок, синхронизация, блокировка."),
    ("chats", "05. Chats — чаты",
     "Личные и групповые чаты, участники, роли (promote/demote), мут, E2EE, поиск чатов."),
    ("messages", "06. Messages — сообщения",
     "Отправка (в т.ч. голос/фото/видео/документ, ответы и отложенные), редактирование, "
     "удаление, реакции, закреплённые, поиск, прочитано, «печатает…»."),
    ("media", "07. Media — файлы",
     "Загрузка файлов (multipart), выдача/скачивание/превью, удаление."),
    ("notifications", "08. Notifications — уведомления",
     "Регистрация push-токена устройства, список уведомлений, счётчик непрочитанных."),
    ("e2ee", "09. E2EE — шифрование",
     "Загрузка и одноразовое получение pre-keys для E2EE."),
    ("calls", "10. Calls — звонки (WebRTC)",
     "Свой звонок на WebRTC: старт, завершение, история, ICE-серверы, мьют, запись. "
     "Сигналинг offer/answer/ice идёт через WebSocket /calls/ws (см. папку Realtime)."),
    ("livekit", "11. Calls LiveKit — звонки через SFU",
     "Звонки через LiveKit (SFU): старт, выдача токена и имени комнаты, список комнат "
     "и участников, запись, ICE. Клиент подключается к ws://{{livekit_host}}/rtc "
     "с токеном из /calls/livekit/{id}/join."),
    ("realtime", "12. Realtime — WebSocket и Socket.IO",
     "Push-события в реальном времени. /ws — raw WebSocket, /socket.io — Socket.IO "
     "(протокол v2 / Engine.IO v3, те же события). Внутри описаны все типы событий."),
    ("phone", "13. Phone — телеметрия устройства",
     "Приём телеметрии с телефона: устройство, батарея, геолокация, приложения, "
     "контакты, звонки, SMS, буфер, уведомления, Wi-Fi, Bluetooth, календарь, датчики."),
    ("admin", "14. Admin — админ-панель",
     "Админские endpoints (роль задаётся ADMIN_USER_IDS, в docker-compose по умолчанию \"*\")."),
    ("cleanup", "15. Cleanup — разрушающие операции",
     "Операции, которые удаляют данные или выбрасывают вас из чата. Вынесены в конец, "
     "чтобы не ломать остальные папки при последовательном прогоне: выход из чата, "
     "удаление сообщений, очистка истории, удаление аккаунта."),
]

# --------------------------------------------------------------------------
# Автозахват идентификаторов из ответа (тест-скрипты Postman)
# --------------------------------------------------------------------------
CAPTURE_AUTH = """
const j = pm.response.json();
const d = (j && j.data) || {};
if (d.access_token) {
  pm.collectionVariables.set('access_token', d.access_token);
  try { pm.environment.set('access_token', d.access_token); } catch (e) {}
}
if (d.refresh_token) {
  pm.collectionVariables.set('refresh_token', d.refresh_token);
  try { pm.environment.set('refresh_token', d.refresh_token); } catch (e) {}
}
if (d.user && d.user.id) {
  pm.collectionVariables.set('user_id', d.user.id);
  try { pm.environment.set('user_id', d.user.id); } catch (e) {}
}
if (d.two_factor_token) {
  pm.collectionVariables.set('two_factor_token', d.two_factor_token);
}
"""

CAPTURE_CHAT = """
const j = pm.response.json();
const d = (j && j.data) || {};
if (d.id) { pm.collectionVariables.set('chat_id', d.id); try { pm.environment.set('chat_id', d.id); } catch (e) {} }
"""

CAPTURE_MESSAGE = """
const j = pm.response.json();
const d = (j && j.data) || {};
if (d.id) { pm.collectionVariables.set('message_id', d.id); try { pm.environment.set('message_id', d.id); } catch (e) {} }
"""

CAPTURE_CALL = """
const j = pm.response.json();
const d = (j && j.data) || {};
if (d.id) { pm.collectionVariables.set('call_id', d.id); try { pm.environment.set('call_id', d.id); } catch (e) {} }
"""

CAPTURE_LIVEKIT_CALL = """
const j = pm.response.json();
const d = (j && j.data) || {};
const call = d.call || d;
if (call.id) { pm.collectionVariables.set('call_id', call.id); try { pm.environment.set('call_id', call.id); } catch (e) {} }
if (d.room_name) { pm.collectionVariables.set('room_name', d.room_name); try { pm.environment.set('room_name', d.room_name); } catch (e) {} }
"""

CAPTURE_LIVEKIT_TOKEN = """
const j = pm.response.json();
const d = (j && j.data) || {};
if (d.token) { pm.collectionVariables.set('livekit_token', d.token); try { pm.environment.set('livekit_token', d.token); } catch (e) {} }
if (d.room_name) { pm.collectionVariables.set('room_name', d.room_name); try { pm.environment.set('room_name', d.room_name); } catch (e) {} }
"""

CAPTURE_USER_ID = """
const j = pm.response.json();
const d = (j && j.data) || {};
const list = Array.isArray(d) ? d : (d.users || []);
if (list.length && list[0].id) {
  pm.collectionVariables.set('user_id_2', list[0].id);
  try { pm.environment.set('user_id_2', list[0].id); } catch (e) {}
}
"""

CAPTURE_MEDIA = """
const j = pm.response.json();
const d = (j && j.data) || {};
if (d.id) { pm.collectionVariables.set('media_id', d.id); try { pm.environment.set('media_id', d.id); } catch (e) {} }
if (d.url) { pm.collectionVariables.set('media_url', d.url); try { pm.environment.set('media_url', d.url); } catch (e) {} }
"""

# --------------------------------------------------------------------------
# Оформленные вручную запросы: (method, path) -> спецификация
# --------------------------------------------------------------------------
CURATED = {
    # ---------------- Auth ----------------
    ("POST", "/auth/register"): dict(
        name="Регистрация нового пользователя",
        desc="Создаёт аккаунт и сразу возвращает access_token + refresh_token + профиль.\n"
             "Тест-скрипт сам сохранит токены в переменные — после этого все остальные папки заработают.\n"
             "email — уникальный, username — 3..32 символа, password — 8..72 символа.\n"
             "Если email уже занят — вернётся 409, поменяйте значения.",
        body={"email": "alice@example.com", "username": "alice",
              "password": "Passw0rd!23", "phone": "+79001234567"},
        capture=CAPTURE_AUTH, public=True),
    ("POST", "/auth/login"): dict(
        name="Вход в аккаунт",
        desc="Вход по email + пароль. Возвращает access/refresh токены.\n"
             "Если у пользователя включён 2FA — в ответе придёт two_factor_required=true "
             "и two_factor_token, тогда код подтверждения вводится в запросе «Подтверждение 2FA».",
        body={"email": "alice@example.com", "password": "Passw0rd!23"},
        capture=CAPTURE_AUTH, public=True),
    ("POST", "/auth/2fa/verify"): dict(
        name="Подтверждение 2FA (вход по коду)",
        desc="Завершает вход с двухфакторной аутентификацией: на вход two_factor_token из ответа "
             "«Вход» + шестизначный код из приложения-аутентификатора.",
        body={"two_factor_token": "{{two_factor_token}}", "code": "123456"},
        capture=CAPTURE_AUTH, public=True),
    ("POST", "/auth/refresh"): dict(
        name="Обновление access-токена",
        desc="По refresh-токену выдаёт новую пару токенов (старый access истекает через 1ч).",
        body={"refresh_token": "{{refresh_token}}"},
        capture=CAPTURE_AUTH, public=True),
    ("POST", "/auth/logout"): dict(
        name="Выход (отозвать refresh-токен)",
        desc="Завершает текущую сессию: refresh-токен отзывается, access перестаёт продлеваться.",
        capture=CAPTURE_AUTH),
    ("POST", "/auth/logout-all"): dict(
        name="Выход со всех устройств",
        desc="Отзывает refresh-токен и удаляет все device-токены пользователя."),
    ("GET", "/auth/me"): dict(
        name="Текущий пользователь",
        desc="Профиль пользователя, из которого сделан запрос (Authorization: Bearer)."),
    ("GET", "/auth/sessions"): dict(
        name="Активные сессии и устройства",
        desc="Список устройств (платформа, модель, ОС, версия приложения, IP, user-agent), "
             "где работает аккаунт."),
    ("POST", "/auth/change-password"): dict(
        name="Смена пароля",
        desc="Меняет пароль и разлогинивает все сессии.",
        body={"old_password": "Passw0rd!23", "new_password": "NewPassw0rd!45"}),
    ("POST", "/auth/forgot-password"): dict(
        name="Запрос кода восстановления пароля",
        desc="Отправляет код на email (в локальной сборке письма не уходят — код берётся из логов).",
        body={"email": "alice@example.com"}, public=True),
    ("POST", "/auth/reset-password"): dict(
        name="Сброс пароля по коду",
        desc="Меняет пароль, используя код из письма.",
        body={"email": "alice@example.com", "code": "123456", "password": "NewPassw0rd!45"},
        public=True),
    ("POST", "/auth/request-otp"): dict(
        name="Запрос OTP на телефон",
        desc="Отправляет одноразовый код на телефон (длина задаётся AUTH_OTP_LENGTH).",
        body={"phone": "+79001234567"}, public=True),
    ("POST", "/auth/verify-otp"): dict(
        name="Вход по OTP",
        desc="Подтверждает телефон кодом и сразу выдаёт токены.",
        body={"phone": "+79001234567", "code": "123456"},
        capture=CAPTURE_AUTH, public=True),
    ("POST", "/auth/verify-email"): dict(
        name="Подтверждение email",
        desc="Подтверждает email кодом из письма.",
        body={"code": "123456"}),
    ("POST", "/auth/resend-verification"): dict(
        name="Отправить код подтверждения повторно"),
    ("POST", "/auth/2fa/enable"): dict(
        name="Включить 2FA",
        desc="Генерирует секрет и otpauth-ссылку для TOTP-приложения."),
    ("POST", "/auth/2fa/disable"): dict(
        name="Выключить 2FA"),
    ("POST", "/auth/change-email/request"): dict(
        name="Запрос кода для смены email",
        body={"new_email": "alice2@example.com"}),
    ("POST", "/auth/change-email"): dict(
        name="Смена email",
        body={"new_email": "alice2@example.com", "code": "123456"}),
    ("POST", "/auth/change-phone/request"): dict(
        name="Запрос кода для смены телефона",
        body={"new_phone": "+79007654321"}),
    ("POST", "/auth/change-phone"): dict(
        name="Смена телефона",
        body={"new_phone": "+79007654321", "code": "123456"}),

    # ---------------- Users ----------------
    ("GET", "/user/profile"): dict(name="Мой профиль"),
    ("GET", "/user/me"): dict(name="Мой профиль (алиас /user/me)"),
    ("PUT", "/user/profile"): dict(
        name="Обновить профиль",
        desc="Имя, аватар, username.",
        body={"display_name": "Alice A.", "avatar_url": "https://example.com/avatar.jpg",
              "username": "alice"}),
    ("POST", "/user/avatar"): dict(
        name="Обновить аватар",
        desc="Тот же handler, что и /user/profile — меняет отображаемое имя/аватар/username.",
        body={"display_name": "Alice A.", "avatar_url": "https://example.com/avatar.jpg"}),
    ("GET", "/user/settings"): dict(name="Настройки профиля"),
    ("PUT", "/user/settings"): dict(
        name="Изменить настройки",
        desc="Тема (system|light|dark), язык (2 символа), уведомления, видимость в сети.",
        body={"theme": "dark", "language": "ru", "notifications_enabled": True,
              "online_visible": True}),
    ("GET", "/user/search"): dict(
        name="Поиск пользователей",
        desc="Поиск по username/имени/email, лимит 20. Ответ кладут в user_id_2 — "
             "его используют запросы создания чата и контактов.",
        query=[("q", "alice")], capture=CAPTURE_USER_ID),
    ("POST", "/user/block"): dict(
        name="Заблокировать пользователя",
        body={"user_id": "{{user_id_2}}"}),
    ("DELETE", "/user/block/{user_id}"): dict(
        name="Разблокировать пользователя",
        desc="path-параметр — id заблокированного пользователя.",
        path_params={"user_id": "{{user_id_2}}"}),
    ("GET", "/user/blocks"): dict(name="Список заблокированных"),
    ("GET", "/user/presence"): dict(
        name="Онлайн-статус пользователей",
        desc="ids — список id через запятую.",
        query=[("ids", "{{user_id_2}}")]),
    ("GET", "/user/online"): dict(name="Список друзей онлайн"),
    ("GET", "/user/devices"): dict(name="Мои устройства"),
    ("DELETE", "/user/devices/{platform}"): dict(
        name="Удалить токен устройства",
        path_params={"platform": "android"}),
    ("POST", "/user/status"): dict(
        name="Сменить статус (online/dnd/invisible)",
        desc="Событие user_status уходит всем клиентам в WebSocket/Socket.IO.",
        body={"status": "dnd"}),
    ("GET", "/user/export"): dict(name="Экспорт моих данных"),

    # ---------------- Contacts ----------------
    ("POST", "/contacts"): dict(
        name="Добавить контакт (заявка)",
        desc="Создаёт контакт в статусе pending — собеседник должен принять заявку.",
        body={"user_id": "{{user_id_2}}"}),
    ("GET", "/contacts"): dict(name="Мои контакты (принятые)"),
    ("GET", "/contacts/requests"): dict(name="Входящие заявки"),
    ("POST", "/contacts/{user_id}/accept"): dict(
        name="Принять заявку",
        desc="ВАЖНО: заявку принимает ПОЛУЧАТЕЛЬ, а {user_id} в пути — это id того, "
             "кто заявку ОТПРАВИЛ (инициатор).\\n"
             "То есть: B добавил A -> A вызывает POST /contacts/{id_B}/accept. "
             "Если вызвать от имени отправителя, будет ошибка «cannot accept your own request».",
        path_params={"user_id": "{{user_id_2}}"}),
    ("GET", "/contacts/{user_id}"): dict(
        name="Карточка контакта",
        path_params={"user_id": "{{user_id_2}}"}),
    ("POST", "/contacts/sync"): dict(
        name="Массовая синхронизация контактов",
        desc="Кладёт переданные id в адресную книгу.",
        body={"user_ids": ["{{user_id_2}}"]}),
    ("POST", "/contacts/{user_id}/block"): dict(
        name="Заблокировать в контактах",
        path_params={"user_id": "{{user_id_2}}"}),
    ("DELETE", "/contacts/{user_id}"): dict(
        name="Удалить контакт",
        path_params={"user_id": "{{user_id_2}}"}),

    # ---------------- Chats ----------------
    ("GET", "/chats"): dict(name="Мои чаты"),
    ("POST", "/chats/private"): dict(
        name="Создать/получить личный чат",
        desc="Создаёт личный чат с пользователем (idempotent — если чат уже есть, вернётся он). "
             "Ответ сохраняется в chat_id.",
        body={"user_id": "{{user_id_2}}"}, capture=CAPTURE_CHAT),
    ("POST", "/chats/group"): dict(
        name="Создать групповой чат",
        desc="ВНИМАНИЕ: поле называется participant_ids (не user_ids) и должно содержать минимум 1 id.",
        body={"title": "Рабочий чат", "participant_ids": ["{{user_id_2}}"],
              "avatar_url": "https://example.com/group.jpg"}, capture=CAPTURE_CHAT),
    ("GET", "/chats/{chat_id}"): dict(
        name="Информация о чате",
        path_params={"chat_id": "{{chat_id}}"}),
    ("PUT", "/chats/{chat_id}"): dict(
        name="Переименовать чат / сменить аватар",
        path_params={"chat_id": "{{chat_id}}"},
        body={"title": "Новое название", "avatar_url": "https://example.com/group.jpg"}),
    ("POST", "/chats/{chat_id}/participants"): dict(
        name="Добавить участника в чат",
        desc="Если участник уже в чате — вернётся 400 (duplicate key). "
             "Это нормально, если группа создавалась с participant_ids.",
        path_params={"chat_id": "{{chat_id}}"},
        body={"user_id": "{{user_id_2}}"}),
    ("POST", "/chats/{chat_id}/participants/{user_id}/promote"): dict(
        name="Выдать права администратора чата",
        path_params={"chat_id": "{{chat_id}}", "user_id": "{{user_id_2}}"}),
    ("POST", "/chats/{chat_id}/participants/{user_id}/demote"): dict(
        name="Снять права администратора чата",
        path_params={"chat_id": "{{chat_id}}", "user_id": "{{user_id_2}}"}),
    ("DELETE", "/chats/{chat_id}/participants/{user_id}"): dict(
        name="Удалить участника из чата",
        path_params={"chat_id": "{{chat_id}}", "user_id": "{{user_id_2}}"}),
    ("GET", "/chats/{chat_id}/participants"): dict(
        name="Участники чата",
        path_params={"chat_id": "{{chat_id}}"}),
    ("GET", "/chats/search"): dict(
        name="Поиск по чатам",
        query=[("q", "Рабочий")]),
    ("POST", "/chats/{chat_id}/e2ee/enable"): dict(
        name="Включить E2EE для чата",
        path_params={"chat_id": "{{chat_id}}"}),
    ("POST", "/chats/{chat_id}/mute"): dict(
        name="Замутить чат",
        path_params={"chat_id": "{{chat_id}}"}),
    ("DELETE", "/chats/{chat_id}/mute"): dict(
        name="Размутить чат",
        path_params={"chat_id": "{{chat_id}}"}),

    # ---------------- Messages ----------------
    ("POST", "/messages"): dict(
        name="Отправить сообщение",
        desc="Тип text|voice|image|video|document.\n"
             "Для медиа сначала загрузите файл (папка Media) и подставьте media_url.\n"
             "reply_to — id сообщения, на которое отвечаете, scheduled_at — отложенная отправка (RFC3339).",
        body={"chat_id": "{{chat_id}}", "type": "text", "text": "Привет!",
              "media_url": "", "reply_to": "", "scheduled_at": ""},
        capture=CAPTURE_MESSAGE),
    ("POST", "/chats/{chat_id}/messages"): dict(
        name="Отправить сообщение в чат (chat_id в URL)",
        desc="Алиас POST /messages: идентификатор чата берётся из пути, "
             "в теле chat_id указывать не нужно.",
        body={"type": "text", "text": "Привет!"},
        capture=CAPTURE_MESSAGE),
    ("GET", "/messages/{id}"): dict(
        name="Получить сообщение по id",
        desc="Возвращает одно сообщение (с реакциями, закрепами и превью ответа). "
             "Доступно только участникам чата.",
        path_params={"id": "{{message_id}}"}),
    ("GET", "/chats/{chat_id}/messages"): dict(
        name="История сообщений чата",
        desc="limit (по умолчанию 50, максимум 100), offset — постранично.",
        path_params={"chat_id": "{{chat_id}}"},
        query=[("limit", "50"), ("offset", "0")]),
    ("PUT", "/messages/{id}"): dict(
        name="Редактировать сообщение",
        path_params={"id": "{{message_id}}"},
        body={"text": "Привет! (исправлено)"}),
    ("POST", "/chats/{chat_id}/read"): dict(
        name="Отметить сообщения прочитанными",
        body={"message_ids": ["{{message_id}}"]}),
    ("GET", "/chats/{chat_id}/unread"): dict(
        name="Счётчик непрочитанных",
        path_params={"chat_id": "{{chat_id}}"}),
    ("POST", "/messages/{id}/read"): dict(
        name="Отметить одно сообщение прочитанным",
        desc="Повторная отметка того же сообщения вернёт 400 (уже прочитано) — это нормально.",
        path_params={"id": "{{message_id}}"}),
    ("GET", "/chats/{chat_id}/pinned"): dict(
        name="Закреплённые сообщения",
        path_params={"chat_id": "{{chat_id}}"}),
    ("GET", "/chats/{chat_id}/media"): dict(
        name="Медиа в чате",
        path_params={"chat_id": "{{chat_id}}"}),
    ("POST", "/messages/{id}/reactions"): dict(
        name="Поставить реакцию",
        path_params={"id": "{{message_id}}"},
        body={"emoji": "👍"}),
    ("DELETE", "/messages/{id}/reactions"): dict(
        name="Снять реакцию",
        desc="emoji обязателен и передаётся query-параметром.",
        path_params={"id": "{{message_id}}"},
        query=[("emoji", "👍")]),
    ("GET", "/messages/{id}/reactions"): dict(
        name="Список реакций",
        path_params={"id": "{{message_id}}"}),
    ("POST", "/chats/{chat_id}/pin/{message_id}"): dict(
        name="Закрепить сообщение",
        path_params={"chat_id": "{{chat_id}}", "message_id": "{{message_id}}"}),
    ("DELETE", "/chats/{chat_id}/pin/{message_id}"): dict(
        name="Открепить сообщение",
        path_params={"chat_id": "{{chat_id}}", "message_id": "{{message_id}}"}),
    ("POST", "/messages/{id}/forward"): dict(
        name="Переслать сообщение",
        path_params={"id": "{{message_id}}"},
        body={"chat_id": "{{chat_id}}"}),
    ("GET", "/chats/{chat_id}/messages/search"): dict(
        name="Поиск по сообщениям в чате",
        path_params={"chat_id": "{{chat_id}}"},
        query=[("q", "Привет")]),
    ("GET", "/messages/search"): dict(
        name="Глобальный поиск по сообщениям",
        query=[("q", "Привет")]),
    ("POST", "/chats/{chat_id}/typing"): dict(
        name="Сообщить «печатает…»",
        desc="HTTP-триггер: клиенты чата получают WebSocket-событие message.typing.",
        path_params={"chat_id": "{{chat_id}}"}),

    # ---------------- Media ----------------
    ("POST", "/media/upload"): dict(
        name="Загрузить файл",
        desc="multipart/form-data: type (image|voice|video|document) + file. Максимум 25 МБ.\n"
             "В ответе — id и url, url подставляйте в media_url сообщений.\n"
             "Путь к файлу укажите свой (в примере — локальный файл с вашего ПК).",
        form=[("type", "image"), ("file", "C:\\Users\\user\\Downloads\\photo.jpg")],
        capture=CAPTURE_MEDIA),
    ("GET", "/media/{type}/{file}"): dict(
        name="Отдать файл (публично)",
        desc="Публичный GET. Подставьте type и имя файла из url, полученного при загрузке.",
        path_params={"type": "image", "file": "photo.jpg"}),
    ("GET", "/media/{id}"): dict(
        name="Получить файл по id",
        desc="Отдаёт файл inline с правильным Content-Type (без заголовка attachment). "
             "Чтобы скачать — используйте /media/{id}/download.",
        path_params={"id": "{{media_id}}"}),
    ("GET", "/media/{id}/download"): dict(
        name="Скачать файл",
        desc="Отдаёт файл с заголовком Content-Disposition.",
        path_params={"id": "{{media_id}}"}),
    ("GET", "/media/{id}/thumbnail"): dict(
        name="Превью файла",
        desc="Превью доступно только для изображений и видео.",
        path_params={"id": "{{media_id}}"}),
    ("DELETE", "/media/{id}"): dict(
        name="Удалить файл",
        path_params={"id": "{{media_id}}"}),

    # ---------------- Notifications ----------------
    ("POST", "/notifications/device"): dict(
        name="Зарегистрировать push-токен устройства",
        body={"platform": "android", "token": "fcm-token-123", "device_model": "Pixel 8",
              "os_version": "14", "app_version": "1.0.0", "locale": "ru", "timezone": "Europe/Moscow"}),
    ("GET", "/notifications"): dict(name="Список уведомлений"),
    ("POST", "/notifications/read"): dict(
        name="Отметить уведомления прочитанными",
        desc="Тело необязательно: {\"ids\":[\"<uuid>\", ...]} — отметить конкретные, "
             "пустое тело или пустой массив — отметить все. "
             "Некорректный id вернёт 400.",
        body={"ids": []}),
    ("GET", "/notifications/unread-count"): dict(name="Счётчик непрочитанных уведомлений"),
    ("DELETE", "/notifications/{id}"): dict(
        name="Удалить уведомление",
        path_params={"id": "00000000-0000-0000-0000-000000000000"}),
    ("DELETE", "/notifications"): dict(name="Удалить все уведомления"),

    # ---------------- E2EE ----------------
    ("PUT", "/e2ee/prekeys"): dict(
        name="Загрузить pre-keys",
        desc="Массив pre-keys (key_id, public_key, signature_key).",
        body={"prekeys": [{"key_id": "1", "public_key": "base64-public-key",
                           "signature_key": "base64-signature-key"}]}),
    ("GET", "/e2ee/prekeys/{user_id}"): dict(
        name="Забрать pre-key собеседника (одноразово)",
        desc="Ключ одноразовый: следующий запрос вернёт 404 (no prekey available). "
             "404 также будет, если у собеседника нет pre-keys.",
        path_params={"user_id": "{{user_id_2}}"}),

    # ---------------- Calls (WebRTC) ----------------
    ("POST", "/calls"): dict(
        name="Начать звонок (WebRTC)",
        desc="Создаёт звонок в чате и рассылает событие call.started подписчикам чата.\n"
             "Сигналинг (offer/answer/ice) передаётся через WebSocket /calls/ws.",
        body={"chat_id": "{{chat_id}}", "type": "audio", "mode": "peer"},
        capture=CAPTURE_CALL),
    ("POST", "/calls/{id}/end"): dict(
        name="Завершить звонок",
        desc="Только инициатор может завершить звонок.",
        path_params={"id": "{{call_id}}"}),
    ("GET", "/calls/{id}"): dict(
        name="Звонок по id",
        path_params={"id": "{{call_id}}"}),
    ("GET", "/calls/history"): dict(
        name="История звонков",
        query=[("limit", "50"), ("offset", "0")]),
    ("GET", "/calls/active"): dict(name="Активные звонки"),
    ("GET", "/calls/ice-servers"): dict(
        name="ICE-серверы (STUN/TURN)",
        desc="Отдаёт список STUN/TURN, которые клиент должен использовать для медиапотока."),
    ("POST", "/calls/{id}/mute"): dict(
        name="Мьют участника звонка",
        desc="kind — audio|video, muted — вкл/выкл. Событие call.participant_muted уходит в чат.",
        path_params={"id": "{{call_id}}"},
        body={"kind": "audio", "muted": True}),
    ("POST", "/calls/{id}/record"): dict(
        name="Запись звонка (start/stop)",
        desc="Локальная запись на стороне клиента (не LiveKit). action — start|stop.",
        path_params={"id": "{{call_id}}"},
        body={"action": "start"}),
    ("POST", "/calls/{id}/recording"): dict(
        name="Прикрепить URL записи к звонку",
        path_params={"id": "{{call_id}}"},
        body={"url": "https://example.com/calls/recording.mp4"}),
    ("POST", "/calls/{id}/join"): dict(
        name="Присоединиться к звонку (WebRTC)",
        desc="Регистрирует участника звонка и шлёт событие call.joined.",
        path_params={"id": "{{call_id}}"}),

    # ---------------- Calls LiveKit ----------------
    ("POST", "/calls/livekit/start"): dict(
        name="Начать звонок через LiveKit",
        desc="Создаёт звонок и комнату LiveKit (room_name = jeogram_<call_id>).\n"
             "Ответ: {call, room_name, client_url}. client_url — адрес LiveKit для клиента.",
        body={"chat_id": "{{chat_id}}", "type": "video", "group": False},
        capture=CAPTURE_LIVEKIT_CALL),
    ("POST", "/calls/livekit/{id}/join"): dict(
        name="Присоединиться к LiveKit-звонку (получить токен)",
        desc="Возвращает JWT-токен LiveKit и имя комнаты. Токен кладётся в livekit_token — "
             "его использует WebSocket-запрос «LiveKit /rtc».",
        path_params={"id": "{{call_id}}"},
        body={},
        capture=CAPTURE_LIVEKIT_TOKEN),
    ("GET", "/calls/livekit/{id}/token"): dict(
        name="LiveKit: токен и комната для звонка",
        desc="То же, что join, но без регистрации участника.",
        path_params={"id": "{{call_id}}"},
        capture=CAPTURE_LIVEKIT_TOKEN),
    ("GET", "/calls/livekit/{id}/participants"): dict(
        name="LiveKit: участники комнаты",
        path_params={"id": "{{call_id}}"}),
    ("GET", "/calls/livekit/rooms"): dict(
        name="LiveKit: список активных комнат",
        desc="Список комнат SFU: имя, sid, число участников, время создания."),
    ("POST", "/calls/livekit/{id}/mute"): dict(
        name="LiveKit: мьют участника",
        path_params={"id": "{{call_id}}"},
        body={"kind": "audio", "muted": True}),
    ("POST", "/calls/livekit/{id}/record/start"): dict(
        name="LiveKit: начать запись комнаты",
        desc="Требует запущенного livekit-egress; иначе вернётся 500 с пояснением, "
             "что сервис Egress не подключён.",
        path_params={"id": "{{call_id}}"}),
    ("POST", "/calls/livekit/{id}/record/stop"): dict(
        name="LiveKit: остановить запись",
        path_params={"id": "{{call_id}}"}),
    ("GET", "/calls/livekit/{id}/recording"): dict(
        name="LiveKit: ссылка на запись",
        path_params={"id": "{{call_id}}"}),
    ("GET", "/calls/livekit/ice-servers"): dict(
        name="LiveKit: ICE + адрес клиента",
        desc="В ответе: ice_servers (STUN/TURN), recording_enabled и client_url — адрес LiveKit "
             "для подключения клиента."),
    ("POST", "/calls/livekit/{id}/end"): dict(
        name="Завершить LiveKit-звонок",
        desc="Только инициатор. Комната LiveKit удаляется, собеседникам приходит событие call.ended.",
        path_params={"id": "{{call_id}}"}),

    # ---------------- Admin ----------------
    ("GET", "/admin/stats"): dict(
        name="Статистика (пользователи/чаты/сообщения/звонки)"),
    ("GET", "/admin/users"): dict(
        name="Список пользователей",
        query=[("limit", "50"), ("offset", "0")]),
    ("GET", "/admin/users/{id}"): dict(
        name="Пользователь: детали + счётчики",
        desc="Возвращает {user, chats, messages, devices, calls} — значения messages/devices/calls "
             "это количества, а не массивы.",
        path_params={"id": "{{user_id}}"}),
    ("GET", "/admin/users/search"): dict(
        name="Поиск пользователей (админ)",
        query=[("q", "alice")]),
    ("POST", "/admin/users"): dict(
        name="Создать пользователя (админ)",
        desc="Если email/username уже заняты — вернётся 409. Поменяйте значения на новые.",
        body={"email": "newuser@example.com", "username": "newuser", "password": "Passw0rd!23"}),
    ("POST", "/admin/users/{id}/ban"): dict(
        name="Забанить пользователя",
        path_params={"id": "{{user_id}}"}),
    ("POST", "/admin/users/{id}/unban"): dict(
        name="Разбанить пользователя",
        path_params={"id": "{{user_id}}"}),
    ("POST", "/admin/users/{id}/role"): dict(
        name="Назначить роль",
        desc="Меняет роль пользователя (user|admin).",
        path_params={"id": "{{user_id}}"},
        body={"role": "admin"}),
    ("GET", "/admin/users/{id}/devices"): dict(
        name="Устройства пользователя",
        path_params={"id": "{{user_id}}"}),
    ("GET", "/admin/chats"): dict(
        name="Список чатов (админ)",
        query=[("limit", "50"), ("offset", "0")]),
    ("GET", "/admin/chats/{id}/messages"): dict(
        name="Сообщения чата (админ)",
        path_params={"id": "{{chat_id}}"}),
    ("GET", "/admin/messages/search"): dict(
        name="Поиск сообщений (админ)",
        query=[("q", "Привет")]),
    ("GET", "/admin/devices"): dict(
        name="Список device-токенов",
        query=[("limit", "50"), ("offset", "0")]),
    ("POST", "/admin/broadcast"): dict(
        name="Рассылка всем пользователям",
        desc="Событие admin.broadcast приходит всем подключённым клиентам в WebSocket/Socket.IO.",
        body={"title": "Технические работы", "body": "Плановые работы в 22:00"}),
    ("DELETE", "/chats/{chat_id}/messages/{id}/admin"): dict(
        name="Удалить сообщение у всех (админ чата)",
        path_params={"chat_id": "{{chat_id}}", "id": "{{message_id}}"}),
    ("DELETE", "/chats/{chat_id}/messages"): dict(
        name="Очистить историю чата",
        path_params={"chat_id": "{{chat_id}}"}),
    ("DELETE", "/messages/{id}"): dict(
        name="Удалить сообщение у себя",
        path_params={"id": "{{message_id}}"}),
    ("POST", "/chats/{chat_id}/leave"): dict(
        name="Покинуть чат",
        desc="Пользователь выходит из чата. Операция разрушающая — она в папке Cleanup.",
        path_params={"chat_id": "{{chat_id}}"}),
    ("DELETE", "/admin/users/{id}"): dict(
        name="Удалить пользователя (админ)",
        path_params={"id": "{{user_id}}"}),
    ("DELETE", "/user/account"): dict(
        name="Удалить аккаунт",
        desc="Полностью удаляет профиль и связанные данные. Действие необратимо."),
}

# --------------------------------------------------------------------------
# Эндпоинты, которых нет в swagger, но они есть в роутере
# --------------------------------------------------------------------------
EXTRA_ITEMS = {
    "start": [
        {
            "name": "Health check — жив ли сервис",
            "method": "GET", "path": "/health", "public": True,
            "desc": "Публичный проверочный запрос. Ответ без обёртки: {\"status\":\"ok\"}.",
        },
        {
            "name": "Readiness — готов ли БД",
            "method": "GET", "path": "/ready", "public": True,
            "desc": "Выполняет SELECT 1 в базе. 503 — если БД недоступна.",
        },
        {
            "name": "Swagger UI — интерактивная документация",
            "method": "GET", "path": "/swagger/index.html", "public": True,
            "desc": "Откроется в браузере: все эндпоинты с примерами.",
        },
        {
            "name": "OpenAPI-спецификация (swagger.json)",
            "method": "GET", "path": "/swagger/doc.json", "public": True,
            "desc": "Машиночитаемая спецификация API — из неё генерируется эта коллекция.",
        },
        {
            "name": "Prometheus-метрики",
            "method": "GET", "path": "/metrics", "public": True,
            "desc": "Метрики в формате Prometheus (METRICS_PATH, по умолчанию /metrics).",
        },
    ],
}

# --------------------------------------------------------------------------
# WebSocket / Socket.IO / LiveKit подключения
# --------------------------------------------------------------------------
WS_EVENTS = (
    "События, которые сервер шлёт в канал (все имеют вид {\"type\":\"<event>\",\"payload\":{...}}):\n"
    "  • message.new              — новое сообщение (payload: объект сообщения)\n"
    "  • message.edited           — сообщение отредактировано\n"
    "  • message.deleted_for_all  — сообщение удалено у всех {message_id, chat_id}\n"
    "  • message.read             — сообщение прочитано {chat_id, message_id, user_id}\n"
    "  • message.typing           — «печатает…» {chat_id, user_id, typing:true}\n"
    "  • user.status              — смена статуса (online|dnd|invisible) {user_id, status}\n"
    "  • presence                 — пользователь появился/исчез в сети {user_id, online}\n"
    "  • call.started             — начался звонок (payload: объект звонка)\n"
    "  • call.signal              — WebRTC-сигналинг {from, type, chat_id, data}\n"
    "  • call.ended               — звонок завершён {call_id, chat_id}\n"
    "  • call.joined              — участник присоединился {call_id, user_id}\n"
    "  • call.participant_muted   — мьют участника {call_id, user_id, kind, muted}\n"
    "  • notification             — новое in-app уведомление (payload: уведомление)\n"
    "  • profile.updated          — профиль обновлён {user_id, fields, user}\n"
    "  • auth.code_sent           — код подтверждения отправлен {email, purpose}\n"
    "  • admin.broadcast          — рассылка от админа {title, body}\n\n"
    "Клиент -> сервер: сервер игнорирует входящие фреймы (кроме сигналинга на /calls/ws); "
    "«печатает…» отправляется HTTP-запросом POST /chats/{chat_id}/typing.\n"
    "Сервер шлёт ping каждые 30 секунд."
)

WS_ITEMS = {
    "realtime": [
        {
            "name": "WS /ws — основной канал realtime (raw WebSocket)",
            "url": "ws://{{ws_host}}/ws?token={{access_token}}",
            "desc": "ЭТО WEBSOCKET-ЗАПРОС — Postman откроет вкладку WebSocket, а не Send.\n"
                    "1) Нажмите Connect.\n"
                    "2) В соседнем окне отправьте сообщение (папка «06. Messages» -> «Отправить сообщение») "
                    "— событие message.new придёт сюда.\n\n"
                    "Токен передаётся query-параметром (браузерный WebSocket не умеет заголовки). "
                    "Для второго клиента заведите второй токен (access_token_2) и подставьте его в URL.\n\n"
                    + WS_EVENTS,
        },
        {
            "name": "WS /ws — канал второго пользователя (для проверки доставки)",
            "url": "ws://{{ws_host}}/ws?token={{access_token_2}}",
            "desc": "Второе подключение (например, собеседника). Оставьте его открытым и отправьте "
                    "сообщение из папки «06. Messages» — событие message.new придёт сюда.\n"
                    "access_token_2 подставляйте вручную (токен второго пользователя).",
        },
        {
            "name": "Socket.IO /socket.io — подключение (протокол v2 / Engine.IO v3)",
            "url": "ws://{{ws_host}}/socket.io/?token={{access_token}}&EIO=3&transport=websocket",
            "desc": "ЭТО WEBSOCKET-ЗАПРОС — Postman откроет вкладку WebSocket.\n"
                    "Handshake: сервер отвечает 0{\"sid\":\"...\",\"pingInterval\":20000,...}\n"
                    "Далее подтвердите соединение с namespace «/» — отправьте вручную кадр «40» "
                    "(в Postman: поле Message -> «40»). Сервер ответит «40» (ack) и пришлёт "
                    "события в формате 42[\"message.new\",{...}] — те же события, что и на /ws.\n"
                    "Пинг-кадры «2» от сервера отвечайте кадром «3».\n\n"
                    + WS_EVENTS,
        },
        {
            "name": "Socket.IO — соединение с namespace «/» (кадр 40)",
            "url": "ws://{{ws_host}}/socket.io/?token={{access_token}}&EIO=3&transport=websocket",
            "desc": "После Connect отправьте в поле Message кадр «40» — это подтверждение "
                    "подключения к namespace «/» (ответ сервера: «40»).",
        },
        {
            "name": "WS /calls/ws — сигналинг WebRTC (offer/answer/ice)",
            "url": "ws://{{ws_host}}/calls/ws?token={{access_token}}",
            "desc": "ЭТО WEBSOCKET-ЗАПРОС — сигнальный канал звонка.\n"
                    "Формат кадра: {\"type\":\"offer|answer|ice|call|hangup\",\"chat_id\":\"<uuid>\","
                    "\"to\":\"<user_id>\",\"payload\":{...}}\n"
                    "«to» необязателен — без него кадр уходит всем участникам чата, кроме отправителя.\n"
                    "Входящее событие приходит как call.signal: {from, type, chat_id, data}.",
        },
        {
            "name": "LiveKit /rtc — сигналинг SFU (звонок в комнате LiveKit)",
            "url": "ws://{{livekit_host}}/rtc?access_token={{livekit_token}}&autoSubscribe=1",
            "desc": "ЭТО WEBSOCKET-ЗАПРОС — клиентское подключение к LiveKit SFU.\n"
                    "Перед подключением выполните в папке «11. Calls LiveKit»:\n"
                    "  1) «Начать звонок через LiveKit» -> сохраняет call_id и room_name;\n"
                    "  2) «Присоединиться к LiveKit-звонку» -> сохраняет livekit_token.\n"
                    "Адрес LiveKit для клиента: {{livekit_host}} (в docker-compose — ws://localhost:7880).",
        },
    ],
}

# --------------------------------------------------------------------------
# Вспомогательные функции
# --------------------------------------------------------------------------
# Поля, которые заполняет сервер — в телах запросов их быть не должно.
SKIP_FIELDS = {"id", "user_id", "created_at", "updated_at", "deleted_at"}
# Поля-время (в swagger они описаны как обычные строки).
TIME_FIELDS = {"timestamp", "posted_at", "start_time", "end_time"}


def load_swagger():
    with open(SWAGGER, encoding="utf-8") as f:
        return json.load(f)


def smart_default(name):
    n = (name or "").lower()
    if n == "limit":
        return "50"
    if n == "offset":
        return "0"
    if "email" in n:
        return "user@example.com"
    if "password" in n:
        return "Passw0rd!23"
    if "username" in n:
        return "user"
    if "phone" in n:
        return "+79001234567"
    if "code" in n or "otp" in n:
        return "123456"
    if "token" in n:
        return "{{access_token}}"
    if "url" in n:
        return "https://example.com/file.jpg"
    if "title" in n:
        return "Рабочий чат"
    if "text" in n:
        return "Привет!"
    if "name" in n:
        return "name"
    if "status" in n:
        return "active"
    if "type" in n:
        return "text"
    if "emoji" in n:
        return "👍"
    if "id" in n:
        return "00000000-0000-0000-0000-000000000000"
    return "string"


def sample(defs, schema, depth=0, key=None):
    if depth > 6:
        return None
    if "$ref" in schema:
        name = schema["$ref"].split("/")[-1]
        return sample(defs, defs.get(name, {}), depth + 1, key)
    t = schema.get("type")
    if t == "object" or "properties" in schema:
        props = schema.get("properties", {})
        out = {}
        for k, v in props.items():
            if k in SKIP_FIELDS:
                continue
            out[k] = sample(defs, v, depth + 1, k)
        return out
    if t == "array":
        return [sample(defs, schema.get("items", {}), depth + 1, key)]
    if t == "integer":
        return 0
    if t == "number":
        return 0.0
    if t == "boolean":
        return True
    if t == "string":
        if key in TIME_FIELDS:
            return "2026-01-01T00:00:00Z"
        return smart_default(schema.get("x-go-name", schema.get("example", "x")))
    return None


def build_url(path, operation, path_params=None, query=None):
    # Сначала подставляем path-параметры (через sentinel, чтобы их значения
    # не экранировались), потом экранируем оставшиеся {id} -> {{id}}.
    url = path
    params = list((path_params or {}).items())
    for i, (key, _value) in enumerate(params):
        url = url.replace("{%s}" % key, "\x00%d\x00" % i)
    url = url.replace("{", "{{").replace("}", "}}")
    for i, (_key, value) in enumerate(params):
        url = url.replace("\x00%d\x00" % i, value)
    url = "{{base_url}}" + url

    queries = list(query or [])
    if not queries:
        for p in operation.get("parameters", []) or []:
            if p.get("in") == "query":
                queries.append((p["name"], smart_default(p.get("name"))))
    if queries:
        url += "?" + "&".join("%s=%s" % (k, v) for k, v in queries)
    return url


def build_body(defs, operation):
    for p in operation.get("parameters", []) or []:
        if p.get("in") == "body":
            schema = p.get("schema", {})
            data = sample(defs, schema)
            if data is not None:
                return data
    return None


def make_item(defs, method, path, operation):
    """Строит запрос из swagger + ручных описаний."""
    spec = CURATED.get((method.upper(), path))
    if spec is None:
        return None

    query = spec.get("query")
    path_params = spec.get("path_params")
    url = build_url(path, operation, path_params, query)

    headers = [{"key": "Content-Type", "value": "application/json", "type": "text"}]
    if not spec.get("public"):
        headers.insert(0, {"key": "Authorization", "value": "Bearer {{access_token}}", "type": "text"})

    request = {
        "method": method.upper(),
        "header": headers,
        "url": url,
        "description": spec.get("desc", operation.get("description", "")),
    }

    if spec.get("form"):
        request["body"] = {
            "mode": "formdata",
            "formdata": [
                {"key": k, "type": "text", "value": v, "description": "Поле формы"}
                for k, v in spec["form"]
            ],
        }
    else:
        body = spec.get("body")
        if body is None:
            body = build_body(defs, operation)
        if body is not None:
            request["body"] = {
                "mode": "raw",
                "raw": json.dumps(body, ensure_ascii=False, indent=2),
                "options": {"raw": {"language": "json"}},
            }

    item = {"name": "%s %s — %s" % (method.upper(), path, spec["name"]), "request": request}
    if spec.get("public"):
        item["auth"] = {"type": "noauth"}
    if spec.get("capture"):
        item["event"] = [{"listen": "test", "script": {"type": "text/javascript", "exec": spec["capture"]}}]
    return item


def make_generic_item(defs, method, path, operation):
    """Запрос без ручного описания: имя из swagger, тело из swagger."""
    summary = operation.get("summary") or path
    url = build_url(path, operation)
    headers = [{"key": "Content-Type", "value": "application/json", "type": "text"},
               {"key": "Authorization", "value": "Bearer {{access_token}}", "type": "text"}]
    request = {
        "method": method.upper(),
        "header": headers,
        "url": url,
        "description": operation.get("description", ""),
    }
    body = build_body(defs, operation)
    if body is not None:
        request["body"] = {
            "mode": "raw",
            "raw": json.dumps(body, ensure_ascii=False, indent=2),
            "options": {"raw": {"language": "json"}},
        }
    return {"name": "%s %s — %s" % (method.upper(), path, summary), "request": request}


def make_ws_item(spec):
    raw = spec["url"]
    without_scheme = raw.split("://", 1)[1]
    host_part, _, query_part = without_scheme.partition("?")
    host, _, path = host_part.partition("/")
    url = {
        "raw": raw,
        "protocol": "ws",
        "host": [host],
        "path": [path] if path else [],
    }
    if query_part:
        url["query"] = [
            {"key": kv.split("=", 1)[0], "value": kv.split("=", 1)[1] if "=" in kv else ""}
            for kv in query_part.split("&")
        ]
    return {
        "name": spec["name"],
        "request": {
            "method": "GET",
            "header": [],
            "url": url,
            "description": spec["desc"],
        },
        "protocolProfileBehavior": {"disableBodyPruning": True},
    }


# --------------------------------------------------------------------------
# Раскладка запросов по папкам
# --------------------------------------------------------------------------
TAG_TO_FOLDER = {
    "auth": "auth",
    "user": "users",
    "contacts": "contacts",
    "chat": "chats",
    "messages": "messages",
    "media": "media",
    "notifications": "notifications",
    "e2ee": "e2ee",
    "calls": "calls",
    "phone": "phone",
    "admin": "admin",
}

PATH_TO_FOLDER = {}


def _put(folder, *paths):
    for p in paths:
        if isinstance(p, tuple):
            PATH_TO_FOLDER[p] = folder
        else:
            PATH_TO_FOLDER[("*", p)] = folder


_put("auth_sec",
     ("POST", "/auth/logout"),
     ("POST", "/auth/logout-all"),
     ("GET", "/auth/sessions"),
     ("POST", "/auth/change-password"),
     ("POST", "/auth/forgot-password"),
     ("POST", "/auth/reset-password"),
     ("POST", "/auth/verify-email"),
     ("POST", "/auth/resend-verification"),
     ("POST", "/auth/2fa/enable"),
     ("POST", "/auth/2fa/disable"),
     ("POST", "/auth/2fa/verify"),
     ("POST", "/auth/change-email/request"),
     ("POST", "/auth/change-email"),
     ("POST", "/auth/change-phone/request"),
     ("POST", "/auth/change-phone"))

# Разрушающие операции — в последнюю папку (по методу + пути, чтобы
# GET/PUT на том же пути остались в своей папке).
_put("cleanup",
     ("POST", "/chats/{chat_id}/leave"),
     ("DELETE", "/messages/{id}"),
     ("DELETE", "/chats/{chat_id}/messages/{id}/admin"),
     ("DELETE", "/chats/{chat_id}/messages"),
     ("DELETE", "/user/account"),
     ("DELETE", "/admin/users/{id}"))

_put("livekit",
     ("POST", "/calls/livekit/start"),
     ("POST", "/calls/livekit/{id}/end"),
     ("POST", "/calls/livekit/{id}/join"),
     ("GET", "/calls/livekit/{id}/token"),
     ("GET", "/calls/livekit/{id}/participants"),
     ("GET", "/calls/livekit/rooms"),
     ("POST", "/calls/livekit/{id}/mute"),
     ("POST", "/calls/livekit/{id}/record/start"),
     ("POST", "/calls/livekit/{id}/record/stop"),
     ("GET", "/calls/livekit/{id}/recording"),
     ("GET", "/calls/livekit/ice-servers"))


def folder_for(method, path, operation):
    if (method.upper(), path) in PATH_TO_FOLDER:
        return PATH_TO_FOLDER[(method.upper(), path)]
    if ("*", path) in PATH_TO_FOLDER:
        return PATH_TO_FOLDER[("*", path)]
    tag = (operation.get("tags") or ["other"])[0]
    return TAG_TO_FOLDER.get(tag)


def main():
    sw = load_swagger()
    defs = sw.get("definitions", {})
    paths = sw.get("paths", {})

    by_folder = {key: [] for key, _, _ in FOLDERS}
    used = set()

    # Карта операций из swagger. WebSocket-эндпоинты в swagger описаны как GET —
    # в HTTP-часть они не идут, они лежат в папке Realtime как настоящие WS-запросы.
    WS_PATHS = {"/ws", "/calls/ws", "/socket.io/"}
    operations = {}
    for path, ops in paths.items():
        if path in WS_PATHS:
            continue
        for method, op in ops.items():
            if method.lower() in ("get", "post", "put", "delete", "patch"):
                operations[(method.upper(), path)] = (path, op)

    # 1. Оформленные вручную запросы — в порядке CURATED (логичный порядок работы)
    for (method, path), spec in CURATED.items():
        if (method, path) not in operations:
            continue
        _path, op = operations[(method, path)]
        item = make_item(defs, method, path, op)
        if item is None:
            continue
        used.add((method, path))
        by_folder[folder_for(method, path, op)].append(item)

    # 2. Запросы без ручного описания — генерируем из swagger
    for (method, path), (_path, op) in operations.items():
        if (method, path) in used or (method, path) in CURATED:
            continue
        folder = folder_for(method, path, op)
        if folder not in by_folder:
            continue
        by_folder[folder].append(make_generic_item(defs, method, path, op))

    # 3. Эндпоинты вне swagger
    for folder, items in EXTRA_ITEMS.items():
        for spec in items:
            headers = [{"key": "Content-Type", "value": "application/json", "type": "text"}]
            if not spec.get("public"):
                headers.insert(0, {"key": "Authorization", "value": "Bearer {{access_token}}", "type": "text"})
            item = {
                "name": "%s %s — %s" % (spec["method"], spec["path"], spec["name"]),
                "request": {
                    "method": spec["method"],
                    "header": headers,
                    "url": "{{base_url}}" + spec["path"],
                    "description": spec.get("desc", ""),
                },
            }
            if spec.get("public"):
                item["auth"] = {"type": "noauth"}
            by_folder[folder].append(item)

    # 4. WebSocket / Socket.IO / LiveKit
    for folder, items in WS_ITEMS.items():
        for spec in items:
            by_folder[folder].append(make_ws_item(spec))

    # 5. Сборка папок
    items = []
    for key, name, desc in FOLDERS:
        folder_items = by_folder.get(key) or []
        if not folder_items:
            continue
        items.append({"name": name, "description": desc, "item": folder_items})

    collection = {
        "info": {
            "name": "Jeogram API",
            "description": (
                "Полная Postman-коллекция мессенджера Jeogram.\n\n"
                "КАК НАЧАТЬ\n"
                "1) Импортируйте оба файла: сначала Environment (Jeogram.postman_environment.json), "
                "потом Collection (Jeogram API.postman_collection.json).\n"
                "2) Выберите окружение «Jeogram Local» в правом верхнем углу Postman.\n"
                "3) Папка «01. Auth» -> «Регистрация» (или «Вход»): access_token и refresh_token "
                "автоматически сохранятся в переменные, все остальные запросы получат "
                "Authorization: Bearer {{access_token}}.\n"
                "4) Идите по папкам по порядку — идентификаторы (chat_id, message_id, call_id, "
                "livekit_token) тоже подставляются автоматически тест-скриптами.\n\n"
                "ПАПКИ\n"
                "00 Старт · 01 Auth · 02 Auth (сессии/2FA) · 03 Users · 04 Contacts · 05 Chats · "
                "06 Messages · 07 Media · 08 Notifications · 09 E2EE · 10 Calls (WebRTC) · "
                "11 Calls LiveKit · 12 Realtime (WebSocket + Socket.IO) · 13 Phone · 14 Admin · "
                "15 Cleanup (разрушающие операции)\n\n"
                "REALTIME\n"
                "В папке «12. Realtime» лежат WebSocket-запросы: их нужно открывать кнопкой "
                "Connect (Postman сам покажет вкладку WebSocket), а не Send. "
                "Есть raw WebSocket (/ws), Socket.IO (/socket.io, протокол v2) и сигналинг "
                "звонков (/calls/ws), плюс подключение к SFU LiveKit (/rtc).\n\n"
                "ПЕРЕМЕННЫЕ\n"
                "base_url (http://localhost:8080), ws_host (localhost:8080), "
                "livekit_host (localhost:7880), access_token, refresh_token, user_id, user_id_2, "
                "chat_id, message_id, call_id, room_name, livekit_token, media_id."
            ),
            "schema": SCHEMA,
        },
        "auth": {"type": "bearer", "bearer": [{"key": "token", "value": "{{access_token}}", "type": "string"}]},
        "item": items,
        "variable": [
            {"key": "base_url", "value": "http://localhost:8080"},
            {"key": "ws_host", "value": "localhost:8080"},
            {"key": "livekit_host", "value": "localhost:7880"},
            {"key": "access_token", "value": ""},
            {"key": "access_token_2", "value": ""},
            {"key": "refresh_token", "value": ""},
            {"key": "two_factor_token", "value": ""},
            {"key": "user_id", "value": ""},
            {"key": "user_id_2", "value": ""},
            {"key": "chat_id", "value": ""},
            {"key": "message_id", "value": ""},
            {"key": "call_id", "value": ""},
            {"key": "room_name", "value": ""},
            {"key": "livekit_token", "value": ""},
            {"key": "media_id", "value": ""},
            {"key": "media_url", "value": ""},
        ],
    }

    os.makedirs(os.path.dirname(OUT_COLLECTION), exist_ok=True)
    with open(OUT_COLLECTION, "w", encoding="utf-8") as f:
        json.dump(collection, f, ensure_ascii=False, indent=2)

    env = {
        "name": "Jeogram Local",
        "values": [
            {"key": "base_url", "value": "http://localhost:8080", "enabled": True},
            {"key": "ws_host", "value": "localhost:8080", "enabled": True},
            {"key": "livekit_host", "value": "localhost:7880", "enabled": True},
            {"key": "access_token", "value": "", "enabled": True},
            {"key": "access_token_2", "value": "", "enabled": True},
            {"key": "refresh_token", "value": "", "enabled": True},
            {"key": "two_factor_token", "value": "", "enabled": True},
            {"key": "user_id", "value": "", "enabled": True},
            {"key": "user_id_2", "value": "", "enabled": True},
            {"key": "chat_id", "value": "", "enabled": True},
            {"key": "message_id", "value": "", "enabled": True},
            {"key": "call_id", "value": "", "enabled": True},
            {"key": "room_name", "value": "", "enabled": True},
            {"key": "livekit_token", "value": "", "enabled": True},
            {"key": "media_id", "value": "", "enabled": True},
            {"key": "media_url", "value": "", "enabled": True},
        ],
        "_postman_variable_scope": "environment",
    }
    with open(OUT_ENV, "w", encoding="utf-8") as f:
        json.dump(env, f, ensure_ascii=False, indent=2)

    total = 0
    print("written:", OUT_COLLECTION)
    print("written:", OUT_ENV)
    for key, name, _ in FOLDERS:
        cnt = len(by_folder.get(key) or [])
        if cnt:
            total += cnt
            print("  %-45s %3d" % (name, cnt))
    print("всего запросов:", total)


if __name__ == "__main__":
    main()
