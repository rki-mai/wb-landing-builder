package auth

import (
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

type User struct {
	ID           string    `json:"id" bson:"_id,omitempty"`
	Email        string    `json:"email" bson:"email"`
	PasswordHash string    `json:"-" bson:"password_hash"`
	CreatedAt    time.Time `json:"created_at" bson:"created_at"`
}

type Me struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type RefreshToken struct {
	ID        string    `bson:"_id,omitempty"`
	UserID    string    `bson:"user_id"`
	Token     string    `bson:"token"`
	ExpiresAt time.Time `bson:"expires_at"`
	CreatedAt time.Time `bson:"created_at"`
}

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// RegisterRequest - запрос на регистрацию
type RegisterRequest struct {
	Body struct {
		Email    string `json:"email" required:"true" example:"user@example.com" doc:"User email address"`
		Password string `json:"password" required:"true" example:"secret123" doc:"User password (min 8 characters)" format:"password"`
	}
}

// LoginRequest - запрос на вход
type LoginRequest struct {
	Body struct {
		Email    string `json:"email" required:"true" example:"user@example.com" doc:"User email address"`
		Password string `json:"password" required:"true" example:"secret123" doc:"User password" format:"password"`
	}
}

// RefreshRequest - запрос на обновление токенов
type RefreshRequest struct {
	Body struct {
		RefreshToken string `json:"refresh_token" required:"true" example:"eyJhbGciOiJIUzI1..." doc:"Refresh token"`
	}
}

// TokenResponse - ответ с токенами
type TokenResponse struct {
	Body struct {
		AccessToken  string `json:"access_token" example:"eyJhbGciOiJIUzI1..." doc:"JWT access token"`
		RefreshToken string `json:"refresh_token" example:"eyJhbGciOiJIUzI1..." doc:"JWT refresh token"`
		ExpiresIn    int64  `json:"expires_in" example:"3600" doc:"Token expiration time in seconds"`
	}
}

// ErrorResponse - ответ с ошибкой
type ErrorResponse struct {
	Body struct {
		Error string `json:"error" example:"Invalid credentials" doc:"Error message"`
	}
}

// MeResponse - ответ с данными текущего пользователя
type MeResponse struct {
	Body struct {
		ID    string `json:"id" example:"507f1f77bcf86cd799439011" doc:"User ID"`
		Email string `json:"email" example:"user@example.com" doc:"User email"`
	}
}

// RegisterResponse - ответ при успешной регистрации
type RegisterResponse struct {
	Body struct {
		ID    string `json:"id" example:"507f1f77bcf86cd799439011" doc:"User ID"`
		Email string `json:"email" example:"user@example.com" doc:"User email"`
	}
}
