<p align="center">
  <img src="docs/assets/hdr-deploy.svg" width="100%" alt="Деплой и эксплуатация"/>
</p>
# Деплой и эксплуатация Jeogram

## 1. База данных: PostgreSQL в Docker (по умолчанию)

По умолчанию поднимается **PostgreSQL в Docker** (сервис `postgres` в
`docker-compose.yml`). Данные хранятся в именованном volume `postgres-data`,
поэтому при пересоздании контейнера история сообщений **не теряется**.

> ⚠️ **SQLite (`sqlite lite`) — это НЕ основной вариант.** Он нужен только для
> локальной разработки без Docker (`DB_DRIVER=sqlite`, файл `jeogram.db`). В
> продакшене и в `docker-compose.yml` используется именно **Postgres**. Не
> путайте: `docker-compose.lite.yml` тоже поднимает **Postgres** (просто без
> Kafka/Prometheus/Grafana), а не SQLite.

Переменные (уже заданы в `docker-compose.yml`, меняйте пароли в `.env`):

```
DB_DRIVER=postgres
POSTGRES_HOST=postgres
POSTGRES_PORT=5432
POSTGRES_USER=jeogram
POSTGRES_PASSWORD=jeogram
POSTGRES_DB=jeogram
POSTGRES_SSLMODE=disable
```

---

## 2. Сколько ресурсов нужно серверу (VPS)

| Состав стека                     | vCPU | RAM   | Диск (SSD)            | Когда |
|----------------------------------|------|-------|-----------------------|-------|
| Только приложение + Postgres     | 2    | 4 ГБ  | 20 ГБ                 | тест/старт, малый трафик |
| Полный стек (Postgres+Redis+Kafka+Prometheus+Grafana) | 4 | 8 ГБ | 40 ГБ | продакшен с метриками и очередью |
| + pgAdmin                        | +0.2 | +0.3 ГБ | —                   | если нужен GUI к БД |

Рекомендация для старта: **2 vCPU / 4 ГБ / 20 ГБ SSD** (Ubuntu 22.04).
Диск под Postgres растёт вместе с сообщениями — закладывайте запас под историю.
Kafka нужна, только если используете асинхронные события/масштабирование; для
одного инстанса realtime работает и без неё (через WebSocket-хаб напрямую).

---

## 3. Шаги деплоя на VPS

```bash
# 1. Ставим Docker + Compose (Ubuntu 22.04)
sudo apt update && sudo apt install -y docker.io docker-compose-plugin
sudo usermod -aG docker $USER   # выйти и зайти заново

# 2. Клонируем проект и настраиваем env
git clone https://github.com/JamikKhidirov/Jeogram-Go.git
cd Jeogram-Go/messenger
cp .env.example .env
# отредактируйте .env: задайте JWT_ACCESS_SECRET/JWT_REFRESH_SECRET (openssl rand -base64 48)
# и смените POSTGRES_PASSWORD / пароли pgAdmin на свои

# 3. Поднимаем весь стек
docker compose up --build -d

# 4. Проверяем
docker compose ps
curl http://localhost:8080/health    # {"status":"ok"}
```

Открыть в браузере:
- API + Swagger: `http://<server-ip>:8080/swagger/index.html`
- Grafana: `http://<server-ip>:3000` (admin/admin)
- Prometheus: `http://<server-ip>:9090`
- **pgAdmin**: `http://<server-ip>:5050` (admin@jeogram.local / admin)

---

## 4. Доступ к PostgreSQL

### Через pgAdmin (GUI, порт 5050)
1. Откройте `http://<server-ip>:5050`, войдите (admin@jeogram.local / admin).
2. Add New Server → General → Name: `jeogram`.
3. Connection → Host: `postgres`, Port: `5432`, Username: `jeogram`,
   Password: `<POSTGRES_PASSWORD>`, Save.
4. Слева: `jeogram` → Databases → `jeogram` → Schemas → Tables — видны все таблицы
   (`users`, `chats`, `chat_participants`, `messages`, `read_receipts`, …).

### Через psql (консоль)
```bash
docker exec -it jeogram-postgres psql -U jeogram -d jeogram
# \dt                    -- список таблиц
# SELECT count(*) FROM messages;
# \q
```

Бэкап/восстановление:
```bash
docker exec jeogram-postgres pg_dump -U jeogram jeogram > dump.sql
docker exec -i jeogram-postgres psql -U jeogram -d jeogram < dump.sql
```

---

## 5. Импорт в Postman (готовая коллекция)

1. Скачайте/склонируйте репозиторий. Импортируйте сначала окружение
   `postman/Jeogram.postman_environment.json`, затем коллекцию
   `postman/Jeogram API.postman_collection.json`.
2. В Postman: **Import** → выберите оба файла. В списке окружений выберите **Jeogram Local**.
3. В коллекции заданы переменные `{{base_url}}` (`http://localhost:8080`) и
   `{{access_token}}` (заполняется автоматически после Login).
4. Порядок проверки:
   - **Auth → Register** → **Auth → Login** (токен захватывается скриптом).
   - Далее любые защищённые запросы работают с `Bearer {{access_token}}`.
   - Realtime: см. папку **Realtime (WebSocket / Socket.IO)** и файл [WEBSOCKET.md](WEBSOCKET.md).
5. Swagger-спецификацию можно импортировать в Postman как OpenAPI:
   `http://<host>:8080/swagger/doc.json` (Postman → Import → Link).

---

## 6. CI/CD

Автоматическая сборка, тесты и сборка образа описаны в
[`.github/workflows/ci.yml`](.github/workflows/ci.yml) (GitHub Actions):
`lint` (go vet) → `test` (`go test ./...`) → `build` (`go build ./...`) →
`docker-build` (сборка образа по Dockerfile). См. раздел ниже и сам файл.

Локально те же шаги:
```bash
make lint        # go vet ./...
make test        # go test ./... -race -count=1
make build       # go build -o bin/server .
make swagger     # перегенерация docs/ (swag init)
make docker-up   # docker compose up --build -d
```

---

## 7. Каталог эндпоинтов (кратко)

Полный список (~60 эндпоинтов) с параметрами — в [ENDPOINTS.md](ENDPOINTS.md) и
в Swagger UI. Группы: **Auth, User, Contacts, Chats, Messages, Notifications,
Media, Calls, Realtime (WebSocket), Phone, E2EE, Admin, Webhooks**.

Все защищённые эндпоинты требуют `Authorization: Bearer <access_token>`.
