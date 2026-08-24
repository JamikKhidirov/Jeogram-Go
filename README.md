# Jeogram — бэкенд мессенджера на Go

Профессиональный бэкенд для мессенджера, написанный на чистой архитектуре
(модульный монолит). Внутренние «микросервисы» общаются через **Kafka**,
данные хранятся в **PostgreSQL**, кэш/сессии — в **Redis**, метрики
собираются **Prometheus** и визуализируются в **Grafana**. Полностью
упаковано в **Docker** и **Kubernetes**.

## Возможности

- **Аутентификация**: регистрация, вход, обновление токена (JWT access/refresh), выход.
- **Пользователи**: профиль, настройки (тема, язык, уведомления, видимость), поиск, блокировки.
- **Чаты**: приватные (1-на-1, дедупликация) и групповые.
- **Групповые чаты и права**: роли `owner` / `admin` / `member`; назначение/понижение
  админов, удаление участников, смена названия/аватара, удаление сообщений «для всех».
- **Сообщения**: текст, голосовые и изображения (загрузка файлов), редактирование,
  удаление, ответы, пересылка, реакции (эмодзи), закрепы, поиск, индикатор печати,
  отметки о прочтении, счётчик непрочитанных.
- **Медиа**: загрузка изображений и голосовых сообщений с валидацией типа/размера.
- **Уведомления**: Kafka-консьюмер, который при новом сообщении
  - сохраняет in-app уведомление,
  - отправляет **push** (iOS/APNs и Android/FCM) в зависимости от платформы устройства,
  - доставляет событие в реальном времени через **WebSocket**.
- **Реалтайм (двойной транспорт)**: сервер доставляет события одновременно
  через **raw WebSocket** (`WS /ws`, удобно тестировать в Postman) и через
  **Socket.IO** (`/socket.io`, протокол v2) — fan-out идёт через единый
  интерфейс `realtime.Broadcaster`. При отключённом Kafka доставка всё равно
  работает напрямую через оба транспорта.
- **Ответ из уведомления**: пуш содержит `chat_id`; клиент (например, Android) может
  сразу отправить ответ `POST /chats/{chat_id}/messages` с текстом — без доп. экранов.
- **Звонки**: WebRTC-сигналинг через WebSocket + запись звонков в БД.
- **Наблюдаемость**: Prometheus-метрики (`/metrics`) + Grafana-дашборд.
- **Документация**: Swagger UI (`/swagger/index.html`) и Postman-коллекция.
- **Тесты**: юнит- и интеграционные тесты (REST-флоу через `httptest`).

## Архитектура

```
cmd/server          -> точка входа, сборка зависимостей (wire вручную)
internal/
  config            -> загрузка конфигурации из .env
  pkg/              -> переиспользуемые библиотеки
    logger          -> zerolog
    database        -> GORM + PostgreSQL / SQLite
    cache           -> Redis (опционально)
    events          -> Kafka producer/consumer (опционально)
    auth            -> JWT + bcrypt
    response        -> единый JSON-конверт ответа
    validator       -> валидация запросов
    middleware      -> auth, логирование, recover, метрики
    ws              -> WebSocket-хаб (реалтайм-доставка)
  modules/          -> бизнес-модули (domain/repository/service/handler)
    auth
    user
    chat            -> чаты + роли/права
    message         -> сообщения + реакции/закрепы/ответы/пересылка/поиск/печать
    media
    notification    -> Kafka-консьюмер + push (iOS/Android)
    calls           -> WebRTC-сигналинг
  server            -> роутер, миграции, монтаж модулей
docs/               -> сгенерированный Swagger
postman/            -> Postman-коллекция
k8s/                -> манифесты Kubernetes (namespace, postgres, redis, kafka, app, ingress)
docker-compose.yml  -> вся инфраструктура
Dockerfile          -> минимальный образ на alpine
prometheus.yml      -> конфиг сбора метрик
```

## Файл `.env` — подробно: где он, что писать, откуда брать значения

Файл `.env` лежит в **корне проекта** (`messenger/.env`). Его нет в репозитории
(он в `.gitignore`), поэтому его нужно создать из шаблона:

```bash
cp .env.example .env
```

Затем открыть `.env` любым редактором и заполнить значения. Ниже — что писать в
каждую переменную и **откуда брать значение**.

### 1. JWT-секреты (ОБЯЗАТЕЛЬНО)
```
JWT_ACCESS_SECRET=любая-длинная-случайная-строка
JWT_REFRESH_SECRET=другая-длинная-случайная-строка
```
- **Откуда взять**: это НЕ внешние ключи. Их придумываете/генерируете вы сами.
  Сгенерировать, например:
  ```bash
  openssl rand -base64 48
  ```
  Скопируйте вывод в `JWT_ACCESS_SECRET`, сгенерируйте ещё раз для `JWT_REFRESH_SECRET`.
  В production держите их в секрете и не коммитьте.

### 2. База данных PostgreSQL
```
DB_DRIVER=postgres
POSTGRES_HOST=postgres        # в Docker Compose — имя сервиса "postgres"
POSTGRES_PORT=5432
POSTGRES_USER=jeogram
POSTGRES_PASSWORD=jeogram      # придумайте свой пароль
POSTGRES_DB=jeogram
POSTGRES_SSLMODE=disable
```
- **Откуда взять**: `POSTGRES_PASSWORD` придумываете вы. В `docker-compose.yml`
  пароль задаётся там же (должен совпадать). В Kubernetes пароль хранится в Secret
  (`k8s/01-secrets.yaml`) — его тоже придумываете вы.
- Для локального запуска БЕЗ Postgres можно использовать SQLite:
  ```
  DB_DRIVER=sqlite
  SQLITE_PATH=./jeogram.db
  REDIS_ENABLED=false
  KAFKA_ENABLED=false
  ```

### 3. Redis (кэш refresh-токенов, сессии)
```
REDIS_ENABLED=true
REDIS_HOST=redis              # в Docker Compose — "redis"
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
```
- Если Redis нет, поставьте `REDIS_ENABLED=false`. Тогда refresh/logout работают
  в stateless-режиме (без отзыва токенов).

### 4. Kafka (асинхронные события)
```
KAFKA_ENABLED=true
KAFKA_BROKERS=kafka:9092       # в Docker Compose — "kafka:9092"
KAFKA_CONSUMER_GROUP=jeogram-notifications
KAFKA_MESSAGE_TOPIC=message.created
KAFKA_NOTIFY_TOPIC=user.notifications
```
- Если Kafka нет, поставьте `KAFKA_ENABLED=false`. Realtime-доставка сообщений
  при этом работает напрямую через WebSocket-хаб.

### 5. Push-уведомления (iOS / Android) — ОПЦИОНАЛЬНО
```
PUSH_ENABLED=false
# Android (Firebase Cloud Messaging):
FCM_SERVER_KEY=
FCM_OAUTH_TOKEN=
# iOS (Apple Push Notification service):
APNS_KEY_ID=
APNS_TEAM_ID=
APNS_KEY_PATH=/data/apns/authkey.p8
APNS_BUNDLE_ID=
APNS_PRODUCTION=false
```
- **Откуда взять FCM**: зайдите в [Firebase Console](https://console.firebase.google.com),
  создайте проект, добавьте приложение Android, в «Project Settings → Cloud Messaging»
  скопируйте **Server key** в `FCM_SERVER_KEY` (либо настройте OAuth-токен).
- **Откуда взять APNs**: в [Apple Developer](https://developer.apple.com),
  создайте Key для APNs, скачайте `.p8`, укажите его путь в `APNS_KEY_PATH`,
  а `APNS_KEY_ID` и `APNS_TEAM_ID` — из консоли Apple.
- **По умолчанию `PUSH_ENABLED=false`** — push не отправляется, а логируется.
  Это позволяет тестировать весь поток без реальных ключей.

### 6. Админ-доступ (опционально)

```
ADMIN_USER_IDS=            # через запятую ID пользователей-админов
# ADMIN_USER_IDS=*         # для личного запуска: админ — любой авторизованный
```

- Свой ID узнаётся через `GET /auth/me` (`data.id`); добавьте его в список и
  перезапустите приложение. Все админ-роуты (`/admin/*`) защищены и недоступны
  посторонним. Подробнее и про телеметрию устройств — в [ADMIN.md](ADMIN.md).

### Где брать Access Token для запросов
1. `POST /auth/register` или `POST /auth/login` — в ответе поле `data.access_token`.
2. Для всех защищённых эндпоинтов добавьте заголовок:
   ```
   Authorization: Bearer <access_token>
   ```
3. Когда access-токен истёк — `POST /auth/refresh` с `refresh_token` из ответа логина.

## Быстрый старт (Docker)

Проект поставляется двумя compose-файлами:

- **`docker-compose.yml`** — **полный стек**: PostgreSQL + Redis + Zookeeper + Kafka
  + Prometheus + Grafana + само приложение. Realtime-доставка идёт через Kafka.
- **`docker-compose.lite.yml`** — **облегчённый стек**: только PostgreSQL + Redis +
  приложение (Kafka/Prometheus/Grafana отключены; realtime работает напрямую
  через WebSocket, так как `KAFKA_ENABLED=false`).

### Полный стек (PostgreSQL + Redis + Kafka + мониторинг)

```bash
cp .env.example .env          # обязательно заполните JWT_ACCESS_SECRET / JWT_REFRESH_SECRET
docker compose up --build -d  # собирает и поднимает ВСЕ сервисы
```

Дождитесь, пока все зависимости станут healthy (Postgres/Redis/Kafka), и приложение
само поднимется после них (`depends_on: condition: service_healthy`).

```bash
docker compose ps             # статус сервисов (все должны быть healthy/running)
docker compose logs -f app    # логи приложения
curl http://localhost:8080/health   # {"status":"ok"}
```

### Облегчённый стек (без Kafka)

```bash
docker compose -f docker-compose.lite.yml up --build -d
```

Остановка и очистка (удаляет тома с данными):

```bash
docker compose down           # либо: docker compose -f docker-compose.lite.yml down
docker compose down -v        # + удалить volumes (БД, uploads, grafana)
```

Сервисы:

| Сервис      | URL / порт                |
|-------------|---------------------------|
| API         | http://localhost:8080     |
| Swagger     | http://localhost:8080/swagger/index.html |
| Health      | http://localhost:8080/health |
| Metrics     | http://localhost:8080/metrics |
| PostgreSQL  | localhost:5432            |
| Redis       | localhost:6379            |
| Kafka       | localhost:9092            |
| Prometheus  | http://localhost:9090     |
| Grafana     | http://localhost:3000 (admin/admin) |
| pgAdmin     | http://localhost:5050 (admin@jeogram.local / admin) |

## Локальный запуск без Docker

```bash
# вариант А: поднять только инфраструктуру в Docker
docker compose up -d postgres redis kafka
cp .env.example .env
go run .

# вариант Б: вообще без внешней инфраструктуры (SQLite)
DB_DRIVER=sqlite SQLITE_PATH=./jeogram.db \
REDIS_ENABLED=false KAFKA_ENABLED=false \
JWT_ACCESS_SECRET=dev-secret JWT_REFRESH_SECRET=dev-secret \
HTTP_PORT=8080 go run .
```

## Запуск в Kubernetes

Манифесты лежат в `k8s/` (namespace, Secret, Postgres, Redis, Kafka, ConfigMap,
Deployment приложения, Service, Ingress, `kustomization.yaml`).

```bash
# 1) Создайте секреты (придумайте свои значения):
kubectl -n jeogram create secret generic jeogram-secrets \
  --from-literal=postgres-password=<ПАРОЛЬ> \
  --from-literal=jwt-access-secret=<СЕКРЕТ> \
  --from-literal=jwt-refresh-secret=<СЕКРЕТ> \
  --from-literal=fcm-server-key=<FCM_КЛЮЧ> \
  --from-literal=apns-key=<СОДЕРЖИМОЕ_APNS_КЛЮЧА>

# 2) Примените всё сразу:
kubectl apply -k k8s/

# 3) Или по файлам (секреты — отдельно!):
kubectl apply -f k8s/00-namespace.yaml
kubectl apply -f k8s/01-secrets.yaml
kubectl apply -f k8s/02-postgres.yaml
kubectl apply -f k8s/03-redis.yaml
kubectl apply -f k8s/04-kafka.yaml        # опционально (KAFKA_ENABLED)
kubectl apply -f k8s/05-configmap.yaml
kubectl apply -f k8s/06-app-deployment.yaml
kubectl apply -f k8s/07-app-service.yaml
kubectl apply -f k8s/08-ingress.yaml
```

- Образ приложения: `jeogram/messenger:latest` (соберите `docker build -t jeogram/messenger:latest .`).
- Ingress рассчитан на `jeogram.local` (добавьте запись в `/etc/hosts` или настройте DNS).
- Kafka в манифестах — одиночный брокер в KRaft-режиме. Если не нужна, удалите
  `k8s/04-kafka.yaml` и поставьте `KAFKA_ENABLED=false` в `k8s/05-configmap.yaml`.

## Групповые чаты и права доступа

Роли участника: `owner` (создатель), `admin` (назначен владельцем), `member`.

| Действие                              | Кто может                     | Эндпоинт (метод) |
|---------------------------------------|------------------------------|------------------|
| Назначить админа                      | только `owner`               | `POST /chats/{id}/participants/{user_id}/promote` |
| Понизить админа                       | только `owner`               | `POST /chats/{id}/participants/{user_id}/demote` |
| Удалить участника                     | `admin`/`owner` (не владельца)| `DELETE /chats/{id}/participants/{user_id}` |
| Сменить название/аватар группы        | `admin`/`owner`              | `PUT /chats/{id}` |
| Удалить сообщение «для всех»          | `admin`/`owner`              | `DELETE /chats/{id}/messages/{message_id}/admin` |
| Добавить участника                    | любой участник               | `POST /chats/{id}/participants` |
| Закрепить/открепить, реакции, печать  | любой участник               | см. ниже |

Пример (владелец `owner` назначает админа):
```bash
curl -X POST http://localhost:8080/chats/<chat_id>/participants/<user_id>/promote \
  -H "Authorization: Bearer <owner_token>"
```

## Реалтайм и ответ из уведомления

Два независимых транспорта доставляют одни и те же события:

- **Raw WebSocket**: `WS /ws` с заголовком `Authorization: Bearer <token>`
  (или query-параметром `?token=`). Удобно тестировать в Postman.
- **Socket.IO (v2)**: `http://localhost:8080` с `path=/socket.io` и
  query-параметром `?token=`. Клиент строго `socket.io-client@2.x`
  (v3/v4 **не совместимы**).

События (оба транспорта): `message.new`, `message.read`, `typing`, `presence`,
`notification`, `call.signal`, `call.started`, `call.ended`. В Socket.IO
комната пользователя — `u:<userID>`.

Подробные гайды: [WEBSOCKET.md](WEBSOCKET.md) (raw WS + Postman) и
[SOCKETIO.md](SOCKETIO.md) (v2-клиент + Node-тест).
- **Ответ из пуш-уведомления (Android)**: пуш содержит поле `chat_id`. Клиент
  сразу отправляет сообщение ответа обычным POST-запросом (без открытия чата):
  ```bash
  curl -X POST http://localhost:8080/chats/<chat_id>/messages \
    -H "Authorization: Bearer <token>" \
    -d '{"chat_id":"<chat_id>","type":"text","text":"Ответ из уведомления"}'
  ```
  Точно так же можно переслать сообщение: `POST /messages/{id}/forward`
  с `{"chat_id":"<целевой chat_id>"}`.

## Основные эндпоинты

```
POST /auth/register
POST /auth/login
POST /auth/refresh
POST /auth/logout
GET  /auth/me

GET  /user/profile
PUT  /user/profile
GET  /user/me                     # алиас профиля (текущий пользователь)
POST /user/avatar                 # алиас обновления профиля
GET  /user/settings
PUT  /user/settings
GET  /user/search?q=
POST /user/block           { "user_id": "..." }
GET  /user/blocks
DELETE /user/block/{user_id}
GET  /user/presence?ids=id1,id2     # онлайн-статус (через WebSocket-хаб)
DELETE /user/account                # удалить свой аккаунт (каскадно)

GET  /contacts                     # список контактов (подтверждённых)
GET  /contacts/requests            # входящие запросы в контакты
GET  /contacts/{user_id}           # запись контакта с пользователем
POST /contacts            { "user_id": "..." }            # отправить запрос
POST /contacts/{user_id}/accept   # принять входящий запрос
DELETE /contacts/{user_id}         # удалить из контактов

GET  /chats
POST /chats/private        { "user_id": "..." }
POST /chats/group          { "title": "...", "participant_ids": [...] }
GET  /chats/{id}           # информация о чате (доступ только участникам)
PUT  /chats/{id}           { "title": "...", "avatar_url": "..." }   # admin/owner
POST /chats/{id}/participants            { "user_id": "..." }
GET  /chats/{id}/participants
POST /chats/{id}/participants/{uid}/promote     # owner
POST /chats/{id}/participants/{uid}/demote      # owner
DELETE /chats/{id}/participants/{uid}           # admin/owner
POST /chats/{id}/mute                          # заглушить чат (для текущего user)
DELETE /chats/{id}/mute                        # снять заглушение чата

POST /messages             { "chat_id": "...", "type": "text|voice|image", "text": "...", "media_url": "...", "reply_to": "..." }
GET  /chats/{id}/messages
PUT  /messages/{id}        { "text": "..." }
DELETE /messages/{id}
POST /chats/{id}/messages/{mid}/admin   # удаление "для всех", admin/owner
POST /chats/{id}/read      { "message_ids": [...] }
GET  /chats/{id}/unread
POST /messages/{id}/read                          # отметить одно сообщение прочитанным
POST /messages/{id}/reactions     { "emoji": "🔥" }
DELETE /messages/{id}/reactions?emoji=🔥
GET  /messages/{id}/reactions
POST /chats/{id}/pin/{mid}
DELETE /chats/{id}/pin/{mid}
GET  /chats/{id}/pinned                           # закреплённые сообщения
GET  /chats/{id}/media                            # медиа (image/voice) чата
DELETE /chats/{id}/messages                       # очистить историю сообщений чата
POST /messages/{id}/forward  { "chat_id": "..." }
GET  /chats/{id}/messages/search?q=
GET  /messages/search?q=
POST /chats/{id}/typing

POST /media/upload         (multipart: type=image|voice, file=...)
GET  /media/{type}/{file}

GET  /notifications
POST /notifications/read
POST /notifications/device { "platform": "ios|android|web", "token": "..." }

POST /calls                { "chat_id": "...", "type": "audio|video" }
POST /calls/{id}/end
GET  /calls/{id}           # запись звонка по id
GET  /calls/history        # история звонков пользователя
WS   /calls/ws             (WebRTC-сигналинг)
WS   /ws                   (real-time сообщения, raw WebSocket)
IO   /socket.io            (real-time сообщения, Socket.IO v2)

# --- Админка (только для ADMIN_USER_IDS, см. ADMIN.md) ---
GET  /admin/users                  # список пользователей (IP, User-Agent)
GET  /admin/users/{id}             # профиль пользователя
GET  /admin/users/{id}/devices     # устройства (модель, ОС, app, локаль, IP)
GET  /admin/chats                  # все чаты
GET  /admin/chats/{id}/messages    # сообщения чата
GET  /admin/messages/search?q=     # глобальный поиск сообщений
GET  /admin/devices                # все устройства (IP, модель, ОС)
GET  /admin/stats                  # агрегированная статистика
```

Все защищённые эндпоинты требуют заголовок `Authorization: Bearer <access_token>`.

## Пуш-уведомления iOS / Android

Каждый клиент регистрирует токен устройства (`POST /notifications/device`) с
платформой `ios` или `android`. При получении события `message.created` из Kafka
консьюмер `notification` отправляет push через соответствующий провайдер:

- **Android / Web** — Firebase Cloud Messaging (`FCM_SERVER_KEY` или `FCM_OAUTH_TOKEN`).
- **iOS** — Apple Push Notification service (`APNS_KEY_ID`, `APNS_TEAM_ID`,
  `APNS_KEY_PATH`, `APNS_BUNDLE_ID`, `APNS_PRODUCTION`).

Если провайдер не сконфигурирован (`PUSH_ENABLED=false`), push просто логируется —
это позволяет тестировать весь поток без реальных ключей. Полезная нагрузка пуша
всегда содержит `chat_id` и `message_id`, чтобы клиент мог сразу открыть чат или
ответить из уведомления.

## Производительность и устойчивость

- Пул соединений БД настроен (PostgreSQL: 25 открытых / 10 idle, lifetime 5 мин).
- Выдача истории чата и поиск сообщений оптимизированы: вместо N+1 запросов
  (по одному на сообщение за реакциями/закрепами/превью ответа) используются
  пакетные выборки — 3 запроса на весь список независимо от его размера.
- Добавлены индексы: `chat_participants(user_id)` (быстрый список чатов
  пользователя и проверка участия) и составной `messages(chat_id, created_at)`
  (быстрая пагинация истории).
- Realtime-доставка дублируется минимально: при включённом Kafka доставку ведёт
  консьюмер, при выключенном — напрямую WebSocket-хаб (без дублей).

## Тесты

```bash
go test ./... -race -count=1
```

Тесты используют in-memory SQLite (pure-Go), поэтому не требуют внешней
инфраструктуры. Покрыты: хеширование/ JWT, валидация, формирование ответов,
регистрация/вход, чаты (включая права админа), отправка/редактирование/реакции/
закрепы/пересылка/поиск/печать сообщений, блокировки и полный HTTP-флоу.

Сквозная проверка всех эндпоинтов доступна скриптом `e2e_test.ps1`
(запускать при поднятом сервере на `http://localhost:8081`).

## Swagger

Генерация спецификации из аннотаций в коде:

```bash
make swagger   # устанавливает swag и перегенерирует docs/
```

Затем откройте http://localhost:8080/swagger/index.html

## Метрики и мониторинг

- `/metrics` отдаёт Prometheus-метрики (количество запросов, латентность по методам/пути).
- Prometheus настроен на сбор с `app:8080` (см. `prometheus.yml`).
- Grafana подключена к Prometheus; добавьте дашборд и используйте `admin/admin`.

## Документация и ссылки

| Что                          | Где смотреть |
|------------------------------|--------------|
| Полный стек (Postgres+Redis+Kafka+Prometheus+Grafana) | [`docker-compose.yml`](docker-compose.yml) |
| Облегчённый стек (Postgres+Redis, без Kafka)          | [`docker-compose.lite.yml`](docker-compose.lite.yml) |
| Переменные окружения (шаблон)                          | [`.env.example`](.env.example) |
| Сборка образа                                               | [`Dockerfile`](Dockerfile) |
| Kubernetes-манифесты (namespace, Postgres, Redis, Kafka, app, ingress) | [`k8s/`](k8s/) · [`k8s/README.md`](k8s/README.md) |
| Postman-коллекция эндпоинтов                                | [`postman/jeogram.postman_collection.json`](postman/jeogram.postman_collection.json) |
| Сквозной e2e-прогон всех эндпоинтов (PowerShell)           | [`e2e_test.ps1`](e2e_test.ps1) |
| Swagger-спецификация (сгенерировано)                       | [`docs/swagger.json`](docs/swagger.json) · UI: `/swagger/index.html` |
| Полный каталог эндпоинтов (55 маршрутов)                  | [`ENDPOINTS.md`](ENDPOINTS.md) |
| Гайд по raw WebSocket + Postman                            | [`WEBSOCKET.md`](WEBSOCKET.md) |
| Гайд по Socket.IO (v2-клиент)                             | [`SOCKETIO.md`](SOCKETIO.md) |
| Деплой (VPS, Postgres, pgAdmin, CI/CD)                    | [`DEPLOYMENT.md`](DEPLOYMENT.md) |
| Админка и телеметрия устройств (Android/iOS)             | [`ADMIN.md`](ADMIN.md) |
| Prometheus-конфиг                                            | [`prometheus.yml`](prometheus.yml) |
| Полезные команды (build/run/test/swagger/docker)            | [`Makefile`](Makefile) |

> Все защищённые эндпоинты требуют заголовок `Authorization: Bearer <access_token>`.
> Полный список эндпоинтов с параметрами — в Swagger UI (`/swagger/index.html`) и в
> разделе «Основные эндпоинты» выше.

## Полезные команды (Makefile)

```bash
make build      # go build -o bin/server .
make run        # go run .
make test       # go test ./... -race -count=1
make swagger    # перегенерация docs/ из аннотаций (swag init)
make docker-up  # docker compose up --build -d
make docker-down
```
