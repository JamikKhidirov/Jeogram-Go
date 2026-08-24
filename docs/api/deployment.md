# Развёртывание (Docker)

Полный стек описан в `docker-compose.yml`. Быстрый старт:

```bash
docker compose up -d --build
```

После старта:

- API: http://localhost:8080
- Swagger UI: http://localhost:8080/swagger/index.html
- PostgreSQL: localhost:5432 (jeogram/jeogram)
- Redis: localhost:6379
- Kafka: localhost:9092 (опционально, см. ниже)

## Минимальный запуск (только API + БД)

Для локальной проверки эндпоинтов достаточно `app` и `postgres`:

```bash
docker compose up -d --build app postgres
```

В `docker-compose.yml` Kafka и Redis **отключены** по умолчанию
(`REDIS_ENABLED=false`, `KAFKA_ENABLED=false`): приложение корректно работает без
них (in-memory rate-limit fallback, реалтайм в процессе). Чтобы включить полный
стек, задайте `REDIS_ENABLED=true`, `KAFKA_ENABLED=true` и верните `depends_on`
для `app` на `redis`/`kafka`.

## Переменные окружения (основные)

| Переменная | Назначение |
| --- | --- |
| `POSTGRES_*` | подключение к БД |
| `JWT_ACCESS_SECRET` / `JWT_REFRESH_SECRET` | секреты JWT (обязательно смените в проде) |
| `REDIS_ENABLED` / `REDIS_*` | кэш/ограничения |
| `KAFKA_ENABLED` / `KAFKA_*` | брокер событий |
| `SMTP_*` | отправка email (коды подтверждения) |
| `AUTH_REQUIRE_EMAIL_VERIFIED` | требовать подтверждение email для входа |
| `ADMIN_USER_IDS` | список admin UUID (`*` = все) |
| `RTC_*` | STUN/TURN для звонков |
| `MESSAGE_EDIT_WINDOW` / `MESSAGE_DELETE_FOR_ALL_WINDOW` | окна редактирования/удаления |

## Миграции

При старте на PostgreSQL применяются версионные миграции из
`internal/pkg/migrate/migrations/*.sql` (таблица `schema_migrations`), затем —
AutoMigrate для создания недостающих таблиц. На SQLite (тесты) используется
только AutoMigrate.

## Здоровье

- `GET /health` → `{"status":"ok"}`
- `GET /ready` → проверка БД.
