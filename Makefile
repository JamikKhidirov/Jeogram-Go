# Полезные команды для разработки и запуска проекта.
.PHONY: build run test tidy swagger docker-up docker-down migrate migrate-down lint

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -race -count=1

tidy:
	go mod tidy

# Генерация Swagger-документации из аннотаций в коде.
swagger:
	go install github.com/swaggo/swag/cmd/swag@latest
	swag init --dir ./cmd,./internal --output ./docs

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

lint:
	go vet ./...
