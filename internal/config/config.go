package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	App      AppConfig
	HTTP     HTTPConfig
	DB       DBConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	Kafka    KafkaConfig
	JWT      JWTConfig
	Media    MediaConfig
	Push     PushConfig
	Metrics  MetricsConfig
	Admin    AdminConfig
	RTC      RTCConfig
	Message  MessageConfig
}

// AdminConfig holds the list of user IDs that are granted admin access.
type AdminConfig struct {
	UserIDs []string
}

type AppConfig struct {
	Name    string
	Env     string
	Version string
}

type HTTPConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DBConfig struct {
	Driver     string // postgres | sqlite
	SQLitePath string
}

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func (p PostgresConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.DBName, p.SSLMode)
}

type RedisConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Password string
	DB       int
}

func (r RedisConfig) Addr() string { return fmt.Sprintf("%s:%d", r.Host, r.Port) }

type KafkaConfig struct {
	Enabled       bool
	Brokers       []string
	ConsumerGroup string
	MessageTopic  string
	NotifyTopic   string
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	Issuer        string
}

type MediaConfig struct {
	UploadDir   string
	MaxFileSize int64
	BaseURL     string
}

type MetricsConfig struct {
	Enabled bool
	Path    string
}

type PushConfig struct {
	Enabled        bool
	FCMServerKey   string
	FCMOAuthToken  string
	APNsKeyID      string
	APNsTeamID     string
	APNsKeyPath    string
	APNsBundleID   string
	APNsProduction bool
}

// RTCConfig holds WebRTC STUN/TURN servers used by calls.
type RTCConfig struct {
	STUNServers []string
	TURNServers []string
	TURNUser    string
	TURNPassword string
	RecordingEnabled bool
	RecordingDir     string
}

// MessageConfig holds message edit/delete-for-all behaviour.
type MessageConfig struct {
	DeleteForAllWindow time.Duration
	EditWindow         time.Duration
}

// Load reads configuration from environment (optionally from a .env file).
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		App: AppConfig{
			Name:    getEnv("APP_NAME", "jeogram"),
			Env:     getEnv("APP_ENV", "development"),
			Version: getEnv("APP_VERSION", "1.0.0"),
		},
		HTTP: HTTPConfig{
			Host:         getEnv("HTTP_HOST", "0.0.0.0"),
			Port:         getEnvInt("HTTP_PORT", 8080),
			ReadTimeout:  getEnvDuration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getEnvDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
		},
		Postgres: PostgresConfig{
			Host:     getEnv("POSTGRES_HOST", "localhost"),
			Port:     getEnvInt("POSTGRES_PORT", 5432),
			User:     getEnv("POSTGRES_USER", "jeogram"),
			Password: getEnv("POSTGRES_PASSWORD", "jeogram"),
			DBName:   getEnv("POSTGRES_DB", "jeogram"),
			SSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
		},
		DB: DBConfig{
			Driver:     getEnv("DB_DRIVER", "postgres"),
			SQLitePath: getEnv("SQLITE_PATH", "./jeogram.db"),
		},
		Redis: RedisConfig{
			Enabled:  getEnvBool("REDIS_ENABLED", true),
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Kafka: KafkaConfig{
			Enabled:       getEnvBool("KAFKA_ENABLED", true),
			Brokers:       getEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
			ConsumerGroup: getEnv("KAFKA_CONSUMER_GROUP", "jeogram-notifications"),
			MessageTopic:  getEnv("KAFKA_MESSAGE_TOPIC", "message.created"),
			NotifyTopic:   getEnv("KAFKA_NOTIFY_TOPIC", "user.notifications"),
		},
		JWT: JWTConfig{
			AccessSecret:  getEnv("JWT_ACCESS_SECRET", "change-me-access"),
			RefreshSecret: getEnv("JWT_REFRESH_SECRET", "change-me-refresh"),
			AccessTTL:     getEnvDuration("JWT_ACCESS_TTL", time.Hour),
			RefreshTTL:    getEnvDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
			Issuer:        getEnv("JWT_ISSUER", "jeogram"),
		},
		Media: MediaConfig{
			UploadDir:   getEnv("MEDIA_UPLOAD_DIR", "./uploads"),
			MaxFileSize: getEnvInt64("MEDIA_MAX_FILE_SIZE", 25<<20), // 25MB
			BaseURL:     getEnv("MEDIA_BASE_URL", "http://localhost:8080/media"),
		},
		Metrics: MetricsConfig{
			Enabled: getEnvBool("METRICS_ENABLED", true),
			Path:    getEnv("METRICS_PATH", "/metrics"),
		},
		Admin: AdminConfig{
			UserIDs: getEnvSlice("ADMIN_USER_IDS", nil),
		},
		RTC: RTCConfig{
			STUNServers:      getEnvSlice("RTC_STUN_SERVERS", []string{"stun:stun.l.google.com:19302"}),
			TURNServers:      getEnvSlice("RTC_TURN_SERVERS", nil),
			TURNUser:         getEnv("RTC_TURN_USER", ""),
			TURNPassword:     getEnv("RTC_TURN_PASSWORD", ""),
			RecordingEnabled: getEnvBool("RTC_RECORDING_ENABLED", false),
			RecordingDir:     getEnv("RTC_RECORDING_DIR", "./recordings"),
		},
		Message: MessageConfig{
			DeleteForAllWindow: getEnvDuration("MESSAGE_DELETE_FOR_ALL_WINDOW", 24*time.Hour),
			EditWindow:         getEnvDuration("MESSAGE_EDIT_WINDOW", 24*time.Hour),
		},
	}

	if cfg.App.Env == "production" && (cfg.JWT.AccessSecret == "change-me-access" || cfg.JWT.RefreshSecret == "change-me-refresh") {
		return nil, fmt.Errorf("jwt secrets must be set in production")
	}
	return cfg, nil
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvInt64(key string, def int64) int64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getEnvSlice(key string, def []string) []string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		parts := []string{}
		cur := ""
		for _, r := range v {
			if r == ',' {
				if cur != "" {
					parts = append(parts, cur)
				}
				cur = ""
				continue
			}
			cur += string(r)
		}
		if cur != "" {
			parts = append(parts, cur)
		}
		if len(parts) > 0 {
			return parts
		}
	}
	return def
}
