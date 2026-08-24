package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/jeogram/messenger/internal/config"
	"github.com/jeogram/messenger/internal/modules/auth/domain"
	"github.com/jeogram/messenger/internal/modules/auth/repository"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/cache"
	"github.com/jeogram/messenger/internal/pkg/mail"
)

// ErrInvalidCredentials is returned when login fails.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrEmailNotVerified is returned when email verification is required.
var ErrEmailNotVerified = errors.New("email not verified")

// AuthService implements authentication use cases.
type AuthService struct {
	repo    *repository.UserRepository
	vrfRepo *repository.VerificationRepository
	jwt     *auth.JWT
	cache   *cache.Redis
	mailer  *mail.Mailer
	authCfg config.AuthConfig
}

func NewAuthService(repo *repository.UserRepository, vrfRepo *repository.VerificationRepository, jwt *auth.JWT, c *cache.Redis, mailer *mail.Mailer, authCfg config.AuthConfig) *AuthService {
	return &AuthService{repo: repo, vrfRepo: vrfRepo, jwt: jwt, cache: c, mailer: mailer, authCfg: authCfg}
}

// generateCode возвращает случайный числовой код заданной длины.
func generateCode(length int) string {
	if length <= 0 {
		length = 6
	}
	const digits = "0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = digits[rand.Intn(len(digits))]
	}
	return string(b)
}

// Register creates a new account, then sends an email verification code.
func (s *AuthService) Register(ctx context.Context, req domain.RegisterRequest) (*domain.AuthResult, error) {
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	u := &domain.User{
		Email:        req.Email,
		Username:     req.Username,
		Phone:        req.Phone,
		PasswordHash: hash,
		DisplayName:  req.Username,
		Status:      domain.StatusActive,
		Role:        domain.RoleUser,
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	s.sendVerification(ctx, u, domain.PurposeEmailVerify)
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
	if u.Status == domain.StatusBanned {
		return nil, errors.New("account banned")
	}
	if s.authCfg.RequireEmailVerified && !u.EmailVerified {
		return nil, ErrEmailNotVerified
	}
	return s.issue(ctx, u)
}

// sendVerification генерирует и отправляет код подтверждения на email.
func (s *AuthService) sendVerification(ctx context.Context, u *domain.User, purpose domain.VerificationPurpose) {
	if u.Email == "" {
		return
	}
	code := generateCode(s.authCfg.OTPLength)
	vc := &domain.VerificationCode{
		UserID:    u.ID,
		Target:    u.Email,
		Purpose:   purpose,
		Code:      code,
		ExpiresAt: time.Now().Add(s.authCfg.CodeTTL),
	}
	_ = s.vrfRepo.Create(ctx, vc)
	_ = s.mailer.SendCode(ctx, u.Email, code, string(purpose))
}

// VerifyEmail подтверждает email по коду из письма.
func (s *AuthService) VerifyEmail(ctx context.Context, userID, code string) error {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	vc, err := s.vrfRepo.FindValid(ctx, u.Email, domain.PurposeEmailVerify, code)
	if err != nil {
		return errors.New("invalid or expired code")
	}
	_ = s.vrfRepo.MarkUsed(ctx, vc.ID)
	u.EmailVerified = true
	return s.repo.Update(ctx, u)
}

// ResendVerification повторно отправляет код подтверждения email.
func (s *AuthService) ResendVerification(ctx context.Context, userID string) error {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.EmailVerified {
		return errors.New("email already verified")
	}
	s.sendVerification(ctx, u, domain.PurposeEmailVerify)
	return nil
}

// RequestPhoneOTP генерирует и «отправляет» OTP на номер телефона.
// Если у пользователя с таким телефоном есть email — код уходит на email,
// иначе (dev-режим) код пишется в лог сервера.
func (s *AuthService) RequestPhoneOTP(ctx context.Context, phone string) error {
	if phone == "" {
		return errors.New("phone required")
	}
	u, err := s.repo.GetByPhone(ctx, phone)
	target := phone
	if err == nil && u.Email != "" {
		target = u.Email
	}
	code := generateCode(s.authCfg.OTPLength)
	vc := &domain.VerificationCode{
		UserID:    uidOrEmpty(u),
		Target:    target,
		Purpose:   domain.PurposePhoneOTP,
		Code:      code,
		ExpiresAt: time.Now().Add(s.authCfg.CodeTTL),
	}
	if err := s.vrfRepo.Create(ctx, vc); err != nil {
		return err
	}
	return s.mailer.SendCode(ctx, target, code, "вход по номеру телефона")
}

// LoginPhone выполняет вход по номеру телефона и OTP-коду.
func (s *AuthService) LoginPhone(ctx context.Context, phone, code string) (*domain.AuthResult, error) {
	u, err := s.repo.GetByPhone(ctx, phone)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	target := phone
	if u.Email != "" {
		target = u.Email
	}
	vc, err := s.vrfRepo.FindValid(ctx, target, domain.PurposePhoneOTP, code)
	if err != nil {
		return nil, errors.New("invalid or expired code")
	}
	_ = s.vrfRepo.MarkUsed(ctx, vc.ID)
	u.PhoneVerified = true
	_ = s.repo.Update(ctx, u)
	return s.issue(ctx, u)
}

func uidOrEmpty(u *domain.User) string {
	if u == nil {
		return ""
	}
	return u.ID
}

// --- Административные действия с пользователями ---

// SetStatus меняет статус аккаунта (active/banned/suspended).
func (s *AuthService) SetStatus(ctx context.Context, id, status string) error {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	u.Status = status
	return s.repo.Update(ctx, u)
}

// SetRole меняет роль пользователя (user/admin).
func (s *AuthService) SetRole(ctx context.Context, id, role string) error {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	u.Role = role
	return s.repo.Update(ctx, u)
}

// DeleteUser удаляет аккаунт по id (soft/native delete).
func (s *AuthService) DeleteUser(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// SearchUsers ищет пользователей по username/display_name/email.
func (s *AuthService) SearchUsers(ctx context.Context, query string, limit int) ([]domain.User, error) {
	users, err := s.repo.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	return users, nil
}

// ForgotPassword инициирует сброс пароля: отправляет код на email.
// Для защиты от перебора возвращает nil даже если email не существует.
func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	u, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil
	}
	code := generateCode(s.authCfg.OTPLength)
	vc := &domain.VerificationCode{
		UserID:    u.ID,
		Target:    u.Email,
		Purpose:   domain.PurposePasswordReset,
		Code:      code,
		ExpiresAt: time.Now().Add(s.authCfg.CodeTTL),
	}
	_ = s.vrfRepo.Create(ctx, vc)
	_ = s.mailer.SendCode(ctx, u.Email, code, "сброс пароля")
	return nil
}

// ResetPassword меняет пароль по email + коду из письма.
func (s *AuthService) ResetPassword(ctx context.Context, email, code, newPassword string) error {
	if len(newPassword) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	u, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return err
	}
	vc, err := s.vrfRepo.FindValid(ctx, u.Email, domain.PurposePasswordReset, code)
	if err != nil {
		return errors.New("invalid or expired code")
	}
	_ = s.vrfRepo.MarkUsed(ctx, vc.ID)
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	u.PasswordHash = hash
	return s.repo.Update(ctx, u)
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
