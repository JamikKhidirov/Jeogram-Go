package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jeogram/messenger/internal/config"
	"golang.org/x/crypto/bcrypt"
)

// Claims is the JWT payload carried in access tokens.
type Claims struct {
	UserID string `json:"uid"`
	Email  string `json:"email"`
	Scope  string `json:"scope,omitempty"`
	jwt.RegisteredClaims
}

// TokenPair holds an access and refresh token.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// JWT issues and verifies tokens.
type JWT struct {
	cfg config.JWTConfig
}

func NewJWT(cfg config.JWTConfig) *JWT { return &JWT{cfg: cfg} }

// GeneratePair creates a new access/refresh token pair for a user.
func (j *JWT) GeneratePair(userID, email string) (TokenPair, error) {
	now := time.Now()
	accessClaims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.cfg.AccessTTL)),
		},
	}
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessStr, err := access.SignedString([]byte(j.cfg.AccessSecret))
	if err != nil {
		return TokenPair{}, err
	}

	refreshClaims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.cfg.RefreshTTL)),
		},
	}
	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshStr, err := refresh.SignedString([]byte(j.cfg.RefreshSecret))
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{AccessToken: accessStr, RefreshToken: refreshStr}, nil
}

// ParseAccess validates an access token and returns its claims.
func (j *JWT) ParseAccess(tokenStr string) (*Claims, error) {
	return parse(tokenStr, j.cfg.AccessSecret)
}

// Generate2FA выпускает короткоживущий challenge-токен для подтверждения 2FA.
// Подписывается access-секретом, но имеет scope="2fa" и TTL 5 минут.
func (j *JWT) Generate2FA(userID, email string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Email:  email,
		Scope:  "2fa",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(j.cfg.AccessSecret))
}

// Parse2FA проверяет challenge-токен 2FA и возвращает claims (scope должен быть "2fa").
func (j *JWT) Parse2FA(tokenStr string) (*Claims, error) {
	c, err := parse(tokenStr, j.cfg.AccessSecret)
	if err != nil {
		return nil, err
	}
	if c.Scope != "2fa" {
		return nil, errors.New("invalid 2fa token")
	}
	return c, nil
}

// ParseRefresh validates a refresh token and returns its claims.
func (j *JWT) ParseRefresh(tokenStr string) (*Claims, error) {
	return parse(tokenStr, j.cfg.RefreshSecret)
}

func parse(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// HashPassword hashes a plaintext password using bcrypt.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword compares a plaintext password with a bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
