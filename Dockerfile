# Этап сборки
FROM golang:1.26 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/server .

# Финальный образ
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates curl

COPY --from=build /app/bin/server /app/server

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --retries=5 \
  CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/server"]
