package auth

import (
	"testing"
	"time"

	"github.com/jeogram/messenger/internal/config"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("supersecret")
	if err != nil {
		t.Fatalf("ошибка хеширования: %v", err)
	}
	if hash == "supersecret" {
		t.Fatal("пароль не должен храниться в открытом виде")
	}
	if !CheckPassword(hash, "supersecret") {
		t.Fatal("пароль не совпадает")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("неверный пароль не должен проходить проверку")
	}
}

func TestJWTGenerateAndParse(t *testing.T) {
	cfg := config.JWTConfig{
		AccessSecret:  "access-secret",
		RefreshSecret: "refresh-secret",
		AccessTTL:     time.Hour,
		RefreshTTL:    24 * time.Hour,
		Issuer:        "jeogram",
	}
	j := NewJWT(cfg)

	pair, err := j.GeneratePair("user-123", "user@example.com")
	if err != nil {
		t.Fatalf("ошибка генерации токена: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("токены не должны быть пустыми")
	}

	claims, err := j.ParseAccess(pair.AccessToken)
	if err != nil {
		t.Fatalf("ошибка разбора access токена: %v", err)
	}
	if claims.UserID != "user-123" || claims.Email != "user@example.com" {
		t.Fatalf("неверные claims: %+v", claims)
	}

	if _, err := j.ParseAccess(pair.RefreshToken); err == nil {
		t.Fatal("refresh токен не должен проходить как access")
	}
}
