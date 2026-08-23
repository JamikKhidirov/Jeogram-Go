package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jeogram/messenger/internal/modules/auth/domain"
	"github.com/jeogram/messenger/internal/modules/auth/repository"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/cache"
)

// ErrInvalidCredentials is returned when login fails.
var ErrInvalidCredentials = errors.New("invalid credentials")

// AuthService implements authentication use cases.
type AuthService struct {
	repo   *repository.UserRepository
	jwt    *auth.JWT
	cache  *cache.Redis
	issuer string
}

func NewAuthService(repo *repository.UserRepository, jwt *auth.JWT, c *cache.Redis) *AuthService {
	return &AuthService{repo: repo, jwt: jwt, cache: c}
}

// Register creates a new account and returns an auth result.
func (s *AuthService) Register(ctx context.Context, req domain.RegisterRequest) (*domain.AuthResult, error) {
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	u := &domain.User{
		Email:        req.Email,
		Username:     req.Username,
		PasswordHash: hash,
		DisplayName:  req.Username,
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	return s.issue(ctx, u)
}

// Login authenticates a user by email + password.
func (s *AuthService) Login(ctx context.Context, req domain.LoginRequest) (*domain.AuthResult, error) {
	u, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		return nil, ErrInvalidCredentials
	}
	return s.issue(ctx, u)
}

// Refresh exchanges a refresh token for a new token pair.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*domain.AuthResult, error) {
	claims, err := s.jwt.ParseRefresh(refreshToken)
	if err != nil {
		return nil, err
	}
	// Если Redis отключён, работаем в stateless-режиме (проверка отзыва пропускается).
	if s.cache != nil {
		stored, ok, err := s.cache.Get(ctx, refreshKey(claims.UserID))
		if err != nil {
			return nil, err
		}
		if !ok || stored != refreshToken {
			return nil, errors.New("refresh token revoked")
		}
	}
	u, err := s.repo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	return s.issue(ctx, u)
}

// Logout revokes the refresh token.
func (s *AuthService) Logout(ctx context.Context, userID string) error {
	if s.cache == nil {
		return nil
	}
	return s.cache.Del(ctx, refreshKey(userID))
}

// Me returns the current user profile.
func (s *AuthService) Me(ctx context.Context, userID string) (*domain.PublicUser, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return domain.ToPublic(u), nil
}

func (s *AuthService) issue(ctx context.Context, u *domain.User) (*domain.AuthResult, error) {
	pair, err := s.jwt.GeneratePair(u.ID, u.Email)
	if err != nil {
		return nil, err
	}
	if s.cache != nil {
		_ = s.cache.Set(ctx, refreshKey(u.ID), pair.RefreshToken, 7*24*time.Hour)
	}
	return &domain.AuthResult{
		User:         domain.ToPublic(u),
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}, nil
}

func refreshKey(userID string) string {
	return fmt.Sprintf("auth:refresh:%s", userID)
}
