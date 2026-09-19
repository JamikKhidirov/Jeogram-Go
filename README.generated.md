# 🚀 Jeogram

<p align="center">
  <img src="docs/assets/hero.svg" width="100%" alt="Jeogram — мессенджер на Go"/>
</p>

<p align="center">
  <a href="https://github.com/JamikKhidirov/Jeogram-Go/stargazers"><img src="https://img.shields.io/github/stars/JamikKhidirov/Jeogram-Go?style=flat" alt="Stars"/></a>
  <a href="https://github.com/JamikKhidirov/Jeogram-Go/network"><img src="https://img.shields.io/github/forks/JamikKhidirov/Jeogram-Go?style=flat" alt="Forks"/></a>
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go"/>
  <img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="MIT"/>
  <a href="https://github.com/JamikKhidirov/Jeogram-Go/actions/workflows/ci.yml"><img src="https://github.com/JamikKhidirov/Jeogram-Go/actions/workflows/ci.yml/badge.svg" alt="CI"/></a>
</p>

**Jeogram** — продакшн-бэкенд мессенджера на Go: чистая архитектура
(`domain → repository → service → handler`), HTTP API и raw WebSocket в реальном
времени, звонки через **LiveKit**, 2FA, сквозное шифрование (E2EE-prekeys),
push-уведомления и админка.

<p align="center"><img src="docs/assets/typing.svg" width="100%" alt="Анимированные возможности Jeogram"/></p>

<p align="center"><img src="docs/assets/divider.svg" width="100%" alt=""/></p>

## ⚡ Быстрый старт

<p align="center">
  <img src="docs/assets/flow.svg" width="100%" alt="5 шагов до первого сообщения"/>
</p>

<details>
<summary><b>Развернуть команды</b></summary>

```bash
# 1. Клон и настройки
git clone https://github.com/JamikKhidirov/Jeogram-Go.git
cd Jeogram-Go
cp .env.example .env
# задайте JWT_ACCESS_SECRET / JWT_REFRESH_SECRET  (openssl rand -base64 48)

# 2. Запуск: API + PostgreSQL
docker compose up -d --build app postgres

# 3. Проверка
curl http://localhost:8080/health                         # {"status":"ok"}
# Swagger UI → http://localhost:8080/swagger/index.html

# 4. Регистрация и вход
curl -X POST localhost:8080/auth/register -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","username":"alice","password":"Passw0rd!23"}'

# 5. Чат и сообщение
curl -X POST localhost:8080/chats/private -H "Authorization: Bearer $TOKEN" -d '{"user_id":"<uuid>"}'
curl -X POST localhost:8080/messages      -H "Authorization: Bearer $TOKEN" -d '{"chat_id":"<uuid>","type":"text","text":"Привет!"}'
```

Без Docker: `DB_DRIVER=sqlite go run .` — локальная SQLite для разработки.

</details>

## 📡 Realtime

<p align="center">
  <img src="docs/assets/realtime.svg" width="100%" alt="Доставка событий в реальном времени"/>
</p>

- Единственный транспорт — **raw WebSocket**: `ws://localhost:8080/ws?token=<access_token>`
  (токен можно передать и заголовком `Authorization: Bearer`).
- Одно событие → все устройства участника: `message.new`, `message.edited`,
<p align="center"><img src="docs/assets/divider.svg" width="100%" alt=""/></p>

## ✨ Возможности

<table>
<tr><td width="50%" valign="top">

**🔐 Доступ**
- JWT access/refresh + ротация, bcrypt
- подтверждение email и вход по OTP
- 2FA (TOTP) с `otpauth://` для QR-кода
- история входов, выход со всех устройств
- смена пароля / email / телефона

**👤 Пользователи**
- профиль, аватар, настройки, поиск
- контакты, заявки, синхронизация телефонной книги
- блокировки, экспорт данных (GDPR)
- статусы `online` / `dnd` / `invisible`

</td><td width="50%" valign="top">

**💬 Чаты и сообщения**
- приватные (идемпотентно) и групповые чаты
- роли `owner` / `admin` / `member`
- текст, голосовые, изображения, видео, документы
- ответы, пересылка, реакции, закрепы, поиск, печать
- отложенная отправка, редактирование и удаление «для всех»
- E2EE: prekey-бандлы и шифротекст на сервере

**📞 Звонки и уведомления**
- LiveKit: аудио/видео, группы до 50, запись
- WebRTC-сигналинг `WS /calls/ws`, STUN/TURN
- push APNs/FCM + in-app лента, webhook-события

</td></tr>
</table>

## 🏗️ Архитектура

<p align="center">
  <img src="docs/assets/architecture.svg" width="100%" alt="Архитектура Jeogram"/>
</p>

<details>
<summary><b>Структура каталогов</b></summary>

```
main.go                  — точка входа
internal/
├── config/              — конфигурация из env
├── server/              — роутер, миграции, монтаж модулей
├── pkg/                 — переиспользуемые библиотеки
│   ├── logger           — zerolog
│   ├── database         — GORM (PostgreSQL / SQLite)
│   ├── cache            — Redis (опционально)
│   ├── events           — Kafka producer/consumer (опционально)
│   ├── auth             — JWT + bcrypt
│   ├── realtime         — Broadcaster (fan-out событий)
│   ├── ws               — WebSocket-хаб
│   ├── mail             — SMTP-отправка кодов
│   ├── ratelimit        — лимиты (Redis / память)
│   ├── middleware       — auth, recover, метрики
│   ├── response         — единый конверт ответа
│   ├── validator        — валидация запросов
│   └── webhook          — исходящие webhook-события
└── modules/             — бизнес-модули
    ├── auth  user  contact  chat  message  media
    ├── notification  calls  phone  e2ee  admin
    └── (в каждом: domain / repository / service / handler)
```

</details>

## 🧰 Технологический стек

<p align="center">
  <img src="docs/assets/stack.svg" width="100%" alt="Технологический стек"/>
</p>
  `message.typing`, `message.read`, `notification`, `call.signal`, `user.status`,
  `admin.broadcast`.
- HTTP-обработчики не зависят от Kafka: если брокер выключен, доставка всё равно
  работает через `realtime.Broadcaster` → `ws.Hub`.

Полное описание — [WEBSOCKET.md](WEBSOCKET.md).
<p align="center"><img src="docs/assets/divider.svg" width="100%" alt=""/></p>

## 📚 Документация

| Раздел | Файл |
| --- | --- |
| 🗂 **Хаб документации** | [docs/README.md](docs/README.md) |
| 🔌 API (все группы) | [docs/api/README.md](docs/api/README.md) |
| 🧾 Swagger UI | http://localhost:8080/swagger/index.html |
| 📬 Postman-коллекция | [postman/Jeogram API.postman_collection.json](postman/Jeogram%20API.postman_collection.json) |
| 📃 Полный каталог эндпоинтов | [ENDPOINTS.md](ENDPOINTS.md) |
| 📡 WebSocket | [WEBSOCKET.md](WEBSOCKET.md) |
| 🛡 Админка | [ADMIN.md](ADMIN.md) |
| 🚢 Деплой и эксплуатация | [DEPLOYMENT.md](DEPLOYMENT.md) |
| 📊 Схема API | [docs/api/deployment.md](docs/api/deployment.md) |

## 🧪 Тесты и качество

```bash
go test ./... -race -count=1   # юнит- и интеграционные тесты (SQLite in-memory)
go vet ./...                   # статический анализ
make lint test build swagger   # основные цели Makefile
go run ./scripts/smoke         # e2e smoke-проверка realtime (13 × [PASS])
```

Все тесты используют in-memory SQLite и не требуют внешней инфраструктуры.
CI (GitHub Actions): `lint → test → build (+swagger check) → docker build → deploy`.

## 🛡️ Безопасность

- JWT с коротким TTL и ротацией refresh-токенов
- bcrypt-хэширование паролей, TOTP-2FA
- валидация всех запросов (`validator`), rate-limiting
- path-traversal-safe загрузка медиа (UUID-имена файлов)
- опциональный HTTPS из коробки (Traefik + Let's Encrypt)

## 📄 Лицензия

MIT © 2024 Jeogram Dev · автор — [JamikKhidirov](https://github.com/JamikKhidirov)

<p align="center">
  <a href="https://github.com/JamikKhidirov/Jeogram-Go/stargazers">
    <img src="docs/assets/star-cta.svg" width="100%" alt="Поставьте звезду проекту Jeogram"/>
  </a>
</p>

<p align="center">
  <img src="docs/assets/divider.svg" width="100%" alt=""/>
  <br>
  <sub>Jeogram — это начало, а не конец. Твоя помощь сделает его лучше! 🌟</sub>
</p>
