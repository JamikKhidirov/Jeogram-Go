# Полезные команды для разработки и запуска проекта.
.PHONY: build run test tidy swagger postman docker-up docker-down migrate migrate-down lint

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

# Генерация понятной Postman-коллекции (все HTTP + WebSocket) из swagger.json.
postman:
	python scripts/genpostman_full.py

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

lint:
	go vet ./...
