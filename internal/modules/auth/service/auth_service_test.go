package service

import (
	"context"
	"testing"

	"github.com/jeogram/messenger/internal/config"
	"github.com/jeogram/messenger/internal/modules/auth/domain"
	"github.com/jeogram/messenger/internal/modules/auth/repository"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/mail"
	"github.com/jeogram/messenger/internal/testutil"
)

func TestAuthService_RegisterLoginMe(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewUserRepository(db)
	jwt := auth.NewJWT(config.JWTConfig{AccessSecret: "a", RefreshSecret: "r", AccessTTL: 0, RefreshTTL: 0})
	vrfRepo := repository.NewVerificationRepository(db)
	mailer := mail.New(config.SMTPConfig{})
	svc := NewAuthService(repo, vrfRepo, jwt, nil, mailer, config.AuthConfig{OTPLength: 6, CodeTTL: 10 * 60e9}, nil)

	ctx := context.Background()
	res, err := svc.Register(ctx, domain.RegisterRequest{
		Email:    "alice@example.com",
		Username: "alice",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("регистрация не удалась: %v", err)
	}
	if res.User.Email != "alice@example.com" {
		t.Fatalf("неверный email: %s", res.User.Email)
	}

	// Повторная регистрация с тем же email должна упасть.
	if _, err := svc.Register(ctx, domain.RegisterRequest{
		Email:    "alice@example.com",
		Username: "alice2",
		Password: "password123",
	}); err == nil {
		t.Fatal("ожидалась ошибка дубликата email")
	}

	// Логин с верным паролем.
	login, err := svc.Login(ctx, domain.LoginRequest{Email: "alice@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("логин не удался: %v", err)
	}
	if login.AccessToken == "" {
		t.Fatal("access token пуст")
	}

	// Логин с неверным паролем.
	if _, err := svc.Login(ctx, domain.LoginRequest{Email: "alice@example.com", Password: "wrong"}); err != ErrInvalidCredentials {
		t.Fatalf("ожидалась ошибка неверных учётных данных, получено: %v", err)
	}

	me, err := svc.Me(ctx, res.User.ID)
	if err != nil {
		t.Fatalf("me не удался: %v", err)
	}
	if me.Username != "alice" {
		t.Fatalf("неверный username: %s", me.Username)
	}
}
