package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jeogram/messenger/internal/config"
	"github.com/jeogram/messenger/internal/modules/auth/domain"
	"github.com/jeogram/messenger/internal/modules/auth/handler"
	"github.com/jeogram/messenger/internal/modules/auth/repository"
	"github.com/jeogram/messenger/internal/modules/auth/service"
	calldomain "github.com/jeogram/messenger/internal/modules/calls/domain"
	callhandler "github.com/jeogram/messenger/internal/modules/calls/handler"
	callrepo "github.com/jeogram/messenger/internal/modules/calls/repository"
	callservice "github.com/jeogram/messenger/internal/modules/calls/service"
	chatdomain "github.com/jeogram/messenger/internal/modules/chat/domain"
	chathandler "github.com/jeogram/messenger/internal/modules/chat/handler"
	chatrepo "github.com/jeogram/messenger/internal/modules/chat/repository"
	chatService "github.com/jeogram/messenger/internal/modules/chat/service"
	mediahandler "github.com/jeogram/messenger/internal/modules/media/handler"
	mediaservice "github.com/jeogram/messenger/internal/modules/media/service"
	messagedomain "github.com/jeogram/messenger/internal/modules/message/domain"
	messagehandler "github.com/jeogram/messenger/internal/modules/message/handler"
	messagerepo "github.com/jeogram/messenger/internal/modules/message/repository"
	messageservice "github.com/jeogram/messenger/internal/modules/message/service"
	notificationdomain "github.com/jeogram/messenger/internal/modules/notification/domain"
	notificationhandler "github.com/jeogram/messenger/internal/modules/notification/handler"
	notificationrepo "github.com/jeogram/messenger/internal/modules/notification/repository"
	notificationservice "github.com/jeogram/messenger/internal/modules/notification/service"
	userdomain "github.com/jeogram/messenger/internal/modules/user/domain"
	userhandler "github.com/jeogram/messenger/internal/modules/user/handler"
	userrepo "github.com/jeogram/messenger/internal/modules/user/repository"
	userservice "github.com/jeogram/messenger/internal/modules/user/service"
	contactdomain "github.com/jeogram/messenger/internal/modules/contact/domain"
	contacthandler "github.com/jeogram/messenger/internal/modules/contact/handler"
	contactrepo "github.com/jeogram/messenger/internal/modules/contact/repository"
	contactservice "github.com/jeogram/messenger/internal/modules/contact/service"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/cache"
	"github.com/jeogram/messenger/internal/pkg/events"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/ws"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"
	"gorm.io/gorm"
)

// Server объединяет HTTP-сервер и его зависимости.
type Server struct {
	cfg      *config.Config
	db       *gorm.DB
	redis    *cache.Redis
	producer *events.Producer
	hub      *ws.Hub
}

// New создаёт сервер: выполняет миграции и инициализирует модули.
func New(cfg *config.Config, db *gorm.DB, redis *cache.Redis, producer *events.Producer, hub *ws.Hub) (*Server, error) {
	if err := migrate(db); err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, db: db, redis: redis, producer: producer, hub: hub}, nil
}

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&domain.User{},
		&userdomain.UserSettings{},
		&userdomain.BlockedUser{},
		&contactdomain.Contact{},
		&chatdomain.Chat{},
		&chatdomain.ChatParticipant{},
		&messagedomain.Message{},
		&messagedomain.ReadReceipt{},
		&messagedomain.Reaction{},
		&messagedomain.PinnedMessage{},
		&notificationdomain.DeviceToken{},
		&notificationdomain.Notification{},
		&calldomain.Call{},
	)
}

// Router builds the chi router with all routes mounted.
func (s *Server) Router() *chi.Mux {
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
	}))
	r.Use(middleware.Logger)
	r.Use(middleware.Recover)
	if s.cfg.Metrics.Enabled {
		r.Use(middleware.Metrics)
	}

	jwtSvc := auth.NewJWT(s.cfg.JWT)

	// Health & readiness.
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/ready", func(w http.ResponseWriter, req *http.Request) {
		if err := s.db.WithContext(req.Context()).Exec("SELECT 1").Error; err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	// Swagger UI.
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	// Realtime websocket.
	r.With(middleware.JWTAuth(jwtSvc)).Get("/ws", s.hub.Handler())

	// Module repositories.
	userRepo := repository.NewUserRepository(s.db)
	settingsRepo := userrepo.NewSettingsRepository(s.db)
	chatRepo := chatrepo.NewChatRepository(s.db)
	msgRepo := messagerepo.NewMessageRepository(s.db)
	deviceRepo := notificationrepo.NewDeviceRepository(s.db)
	notifRepo := notificationrepo.NewNotificationRepository(s.db)
	callRepo := callrepo.NewCallRepository(s.db)
	contactRepo := contactrepo.NewContactRepository(s.db)

	// Module services.
	authSvc := service.NewAuthService(userRepo, jwtSvc, s.redis)
	userSvc := userservice.NewUserService(userRepo, settingsRepo, contactRepo, chatRepo)
	chatSvc := chatService.NewChatService(chatRepo)
	msgSvc := messageservice.NewMessageService(msgRepo, chatRepo, s.producer, s.cfg.Kafka, s.hub)
	mediaSvc, err := mediaservice.NewMediaService(s.cfg.Media)
	if err != nil {
		panic(fmt.Sprintf("media service: %v", err))
	}
	pushSvc := notificationservice.NewPushService(s.cfg.Push)
	notifSvc := notificationservice.NewNotificationService(deviceRepo, notifRepo, userRepo, chatRepo, pushSvc, s.hub, s.producer, s.cfg.Kafka.NotifyTopic)
	callSvc := callservice.NewCallService(callRepo, chatRepo)
	contactSvc := contactservice.NewContactService(contactRepo, userRepo)

	// Module handlers.
	authH := handler.NewAuthHandler(authSvc, jwtSvc)
	userH := userhandler.NewUserHandler(userSvc, jwtSvc, s.hub)
	chatH := chathandler.NewChatHandler(chatSvc, jwtSvc)
	msgH := messagehandler.NewMessageHandler(msgSvc, jwtSvc)
	mediaH := mediahandler.NewMediaHandler(mediaSvc, jwtSvc)
	notifH := notificationhandler.NewNotificationHandler(notifSvc, jwtSvc)
	callH := callhandler.NewCallsHandler(callSvc, chatRepo, s.hub, jwtSvc)
	contactH := contacthandler.NewContactHandler(contactSvc, jwtSvc)

	authH.RegisterRoutes(r)
	userH.RegisterRoutes(r)
	chatH.RegisterRoutes(r)
	msgH.RegisterRoutes(r)
	mediaH.RegisterRoutes(r)
	notifH.RegisterRoutes(r)
	callH.RegisterRoutes(r)
	contactH.RegisterRoutes(r)

	if s.cfg.Metrics.Enabled {
		r.Handle(s.cfg.Metrics.Path, promhttp.Handler())
	}

	return r
}

// Run starts the HTTP server and blocks until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.HTTP.Host, s.cfg.HTTP.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.Router(),
		ReadTimeout:  s.cfg.HTTP.ReadTimeout,
		WriteTimeout: s.cfg.HTTP.WriteTimeout,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
