package cache

import (
	"context"
	"time"

	"github.com/jeogram/messenger/internal/config"
	"github.com/redis/go-redis/v9"
)

// Redis wraps a go-redis client with helper methods used across modules.
type Redis struct {
	client *redis.Client
}

// NewRedis connects to Redis using the provided configuration.
func NewRedis(cfg config.RedisConfig) (*Redis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     25,
		MinIdleConns: 5,
		MaxRetries:   3,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Redis{client: client}, nil
}

func (r *Redis) Client() *redis.Client { return r.client }

// Set stores a value with a TTL. Use 0 for no expiration.
func (r *Redis) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

// Get returns the value for a key, or ("", false) if missing.
func (r *Redis) Get(ctx context.Context, key string) (string, bool, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

// Del removes one or more keys.
func (r *Redis) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Exists reports whether a key is present.
func (r *Redis) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.client.Exists(ctx, key).Result()
	return n > 0, err
}

// Close closes the underlying connection.
func (r *Redis) Close() error { return r.client.Close() }

// SetSession stores a session entry (userID -> device/platform info) with TTL.
func (r *Redis) SetSession(ctx context.Context, userID, sessionID, platform string, ttl time.Duration) error {
	key := "session:" + sessionID
	val := platform
	return r.Set(ctx, key, val, ttl)
}

// GetSession retrieves a session by sessionID.
func (r *Redis) GetSession(ctx context.Context, sessionID string) (string, bool, error) {
	return r.Get(ctx, "session:"+sessionID)
}

// DeleteSession removes a session entry.
func (r *Redis) DeleteSession(ctx context.Context, sessionID string) error {
	return r.Del(ctx, "session:"+sessionID)
}

// SetUserStatus stores the presence status of a user.
func (r *Redis) SetUserStatus(ctx context.Context, userID, status string, ttl time.Duration) error {
	return r.Set(ctx, "presence:"+userID, status, ttl)
}

// GetUserStatus retrieves the presence status of a user.
func (r *Redis) GetUserStatus(ctx context.Context, userID string) (string, bool, error) {
	return r.Get(ctx, "presence:" + userID)
}

// SetCall stores a call record in Redis for fast lookup.
func (r *Redis) SetCall(ctx context.Context, callID string, data []byte, ttl time.Duration) error {
	return r.Set(ctx, "call:"+callID, string(data), ttl)
}

// GetCall retrieves a call record from Redis.
func (r *Redis) GetCall(ctx context.Context, callID string) (string, bool, error) {
	return r.Get(ctx, "call:" + callID)
}

// SetRecordingURL stores the recording URL for a call.
func (r *Redis) SetRecordingURL(ctx context.Context, callID, url string) error {
	return r.Set(ctx, "recording:"+callID, url, 7*24*time.Hour)
}

// GetRecordingURL retrieves the recording URL for a call.
func (r *Redis) GetRecordingURL(ctx context.Context, callID string) (string, bool, error) {
	return r.Get(ctx, "recording:" + callID)
}

// SetOnline marks a user as online and stores their connection info.
func (r *Redis) SetOnline(ctx context.Context, userID string, ttl time.Duration) error {
	return r.Set(ctx, "online:"+userID, "1", ttl)
}

// IsOnline checks whether a user is currently online.
func (r *Redis) IsOnline(ctx context.Context, userID string) (bool, error) {
	_, ok, err := r.Get(ctx, "online:"+userID)
	if err == redis.Nil || !ok {
		return false, nil
	}
	return true, err
}

// DeleteOnline marks a user as offline.
func (r *Redis) DeleteOnline(ctx context.Context, userID string) error {
	return r.Del(ctx, "online:"+userID)
}
