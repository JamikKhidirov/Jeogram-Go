# Полезные команды для разработки и запуска проекта.
.PHONY: build run test tidy swagger postman postman-test realtime-test docker-up docker-down docker-up-lite migrate migrate-down lint

build:
	go build -o bin/server .

run:
	go run .

test:
	go test ./... -race -count=1

tidy:
	go mod tidy

# Генерация Swagger-документации из аннотаций в коде.
swagger:
	go install github.com/swaggo/swag/cmd/swag@latest
	swag init --dir .,./internal --exclude ./docs --output ./docs --parseDependency --parseInternal

# Генерация понятной Postman-коллекции (HTTP + WebSocket + Socket.IO + LiveKit) из swagger.json.
postman:
	python scripts/genpostman_full.py

# Прогон Postman-коллекции по поднятому стенду (нужен запущенный docker compose).
postman-test:
	go run ./scripts/collectiontest

# Проверка realtime: доставка событий в /ws и /socket.io.
realtime-test:
	go run ./scripts/wsprobe

docker-up:
	docker compose up --build -d

# Облегчённый стек (postgres + redis + livekit + app) — для машин с 4-8 ГБ RAM.
docker-up-lite:
	docker compose -f docker-compose.lite.yml up --build -d

docker-down:
	docker compose down

lint:
	go vet ./...
