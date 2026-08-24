package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/jeogram/messenger/docs"
	"github.com/jeogram/messenger/internal/config"
	authrepo "github.com/jeogram/messenger/internal/modules/auth/repository"
	chatrepo "github.com/jeogram/messenger/internal/modules/chat/repository"
	notificationrepo "github.com/jeogram/messenger/internal/modules/notification/repository"
	notificationservice "github.com/jeogram/messenger/internal/modules/notification/service"
	"github.com/jeogram/messenger/internal/pkg/cache"
	"github.com/jeogram/messenger/internal/pkg/database"
	"github.com/jeogram/messenger/internal/pkg/events"
	"github.com/jeogram/messenger/internal/pkg/logger"
	"github.com/jeogram/messenger/internal/pkg/ws"
	appserver "github.com/jeogram/messenger/internal/server"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// openDB выбирает драйвер БД в зависимости от конфигурации (postgres или sqlite).
func openDB(ctx context.Context, cfg *config.Config) (*gorm.DB, error) {
	if cfg.DB.Driver == "sqlite" {
		log.Info().Str("path", cfg.DB.SQLitePath).Msg("используется SQLite")
		return database.NewSQLite(cfg.DB.SQLitePath, cfg.App.Env == "development")
	}
	log.Info().Msg("используется PostgreSQL")
	return database.NewPostgres(ctx, cfg.Postgres, cfg.App.Env == "development")
}

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Введите токен в формате: Bearer <token>
func init() {
	docs.SwaggerInfo.Title = "Jeogram Messenger API"
	docs.SwaggerInfo.Description = "Профессиональный бэкенд мессенджера на Go (модульный монолит)."
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.Host = "localhost:8080"
	docs.SwaggerInfo.Schemes = []string{"http"}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log.Logger = logger.New(cfg.App.Env, cfg.App.Version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := openDB(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}

	var redis *cache.Redis
	if cfg.Redis.Enabled {
		redis, err = cache.NewRedis(cfg.Redis)
		if err != nil {
			log.Fatal().Err(err).Msg("redis connection failed")
		}
		defer redis.Close()
		log.Info().Msg("connected to redis")
	} else {
		log.Info().Msg("redis отключён (REDIS_ENABLED=false)")
	}

	var producer *events.Producer
	if cfg.Kafka.Enabled {
		events.EnsureTopics(cfg.Kafka)
		producer = events.NewProducer(cfg.Kafka)
		defer producer.Close()
		log.Info().Msg("kafka producer готов")
	} else {
		log.Info().Msg("kafka отключена (KAFKA_ENABLED=false)")
	}

	hub := ws.NewHub()
	go hub.Run()

	server, err := appserver.New(cfg, db, redis, producer, hub)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to build server")
	}

	// Фоновые воркеры (публикация отложенных сообщений и т.п.).
	server.StartWorkers(ctx)

	// Notification microservice (Kafka consumer) inside the monolith.
	userRepo := authrepo.NewUserRepository(db)
	chatRepo := chatrepo.NewChatRepository(db)
	deviceRepo := notificationrepo.NewDeviceRepository(db)
	notifRepo := notificationrepo.NewNotificationRepository(db)
	pushSvc := notificationservice.NewPushService(cfg.Push)
	notifSvc := notificationservice.NewNotificationService(deviceRepo, notifRepo, userRepo, chatRepo, pushSvc, hub, producer, cfg.Kafka.NotifyTopic)
	if cfg.Kafka.Enabled {
		consumer := events.NewConsumer(cfg.Kafka, cfg.Kafka.MessageTopic)
		go notifSvc.Run(ctx, consumer)
	}

	log.Info().Int("port", cfg.HTTP.Port).Msg("starting jeogram server")
	printBanner(cfg)

	if err := server.Run(ctx); err != nil {
		log.Fatal().Err(err).Msg("server stopped with error")
	}
	log.Info().Msg("server shut down gracefully")
}

// printBanner выводит в терминал приветствие с готовыми ссылками на сервисы.
func printBanner(cfg *config.Config) {
	host := cfg.HTTP.Host
	if host == "0.0.0.0" || host == "" {
		host = "localhost"
	}
	base := fmt.Sprintf("http://%s:%d", host, cfg.HTTP.Port)
	line := "------------------------------------------------------------"
	links := [][2]string{
		{"API", base + "/"},
		{"Swagger UI", base + "/swagger/index.html"},
		{"Health", base + "/health"},
		{"Metrics (Prometheus)", base + "/metrics"},
		{"WebSocket (realtime)", "ws://" + host + ":" + strconv.Itoa(cfg.HTTP.Port) + "/ws"},
	}
	fmt.Fprintln(os.Stdout, line)
	fmt.Fprintln(os.Stdout, "  Jeogram Messenger API — сервер запущен")
	fmt.Fprintln(os.Stdout, line)
	for _, l := range links {
		fmt.Fprintf(os.Stdout, "  %-22s %s\n", l[0]+":", l[1])
	}
	fmt.Fprintln(os.Stdout, line)
	fmt.Fprintln(os.Stdout, "  Открой Swagger UI в браузере, чтобы увидеть все эндпоинты.")
	fmt.Fprintln(os.Stdout, line)
}
