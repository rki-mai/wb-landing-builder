package service

import (
	"context"
	"errors"
	"time"

	"github.com/rki-mai/wb-landing-builder/auth/models"
	"github.com/rki-mai/wb-landing-builder/auth/repository"
	"github.com/rki-mai/wb-landing-builder/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo       repository.AuthRepository
	jwtSecret  string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewAuthService(
	repo repository.AuthRepository,
	cfg *config.Config,
) *AuthService {
	return &AuthService{
		repo:       repo,
		jwtSecret:  cfg.JWTSecret,
		accessTTL:  cfg.JWTExpiration,
		refreshTTL: cfg.RefreshTokenExpiration,
	}
}

func (s *AuthService) Register(ctx context.Context, email, password string) (*models.User, error) {
	existing, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("user already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*models.TokenResponse, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		return nil, errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	return s.generateTokens(ctx, user.ID)
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*models.TokenResponse, error) {
	stored, err := s.repo.GetRefreshToken(ctx, refreshToken)
	if err != nil || stored == nil {
		return nil, errors.New("invalid refresh token")
	}

	if time.Now().After(stored.ExpiresAt) {
		return nil, errors.New("refresh token expired")
	}

	if err := s.repo.DeleteRefreshToken(ctx, refreshToken); err != nil {
		return nil, err
	}

	return s.generateTokens(ctx, stored.UserID)
}

func (s *AuthService) generateTokens(ctx context.Context, userID string) (*models.TokenResponse, error) {
	now := time.Now()

	claims := models.Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	refreshToken := uuid.NewString()

	rt := &models.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    userID,
		Token:     refreshToken,
		ExpiresAt: now.Add(s.refreshTTL),
		CreatedAt: now,
	}

	if err := s.repo.SaveRefreshToken(ctx, rt); err != nil {
		return nil, err
	}

	return &models.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
	}, nil
}

func (s *AuthService) GetUserFromToken(tokenStr string) (string, error) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&models.Claims{},
		func(token *jwt.Token) (interface{}, error) {

			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}

			return []byte(s.jwtSecret), nil
		},
	)

	if err != nil || !token.Valid {
		return "", errors.New("invalid token")
	}

	claims, ok := token.Claims.(*models.Claims)
	if !ok {
		return "", errors.New("invalid claims")
	}

	return claims.UserID, nil
}

func (s *AuthService) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	user, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}
	return user, nil
}
