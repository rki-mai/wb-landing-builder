package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// User представляет собой модель пользователя в системе.
type User struct {
	// ID уникальный идентификатор пользователя.
	ID string `json:"id" bson:"_id,omitempty" example:"507f191e810c19729de860ea"`
	// Email адрес электронной почты пользователя.
	Email string `json:"email" bson:"email" example:"newuser@example.com"`
	// PasswordHash хэш пароля (не возвращается в API ответах).
	PasswordHash string `json:"-" bson:"password_hash"`
	// CreatedAt дата и время создания аккаунта.
	CreatedAt time.Time `json:"created_at" bson:"created_at" example:"2023-10-01T12:00:00Z"`
}

// Me представляет собой информацию о пользователе.
type Me struct {
	// ID уникальный идентификатор пользователя.
	ID string `json:"id" bson:"_id,omitempty" example:"507f191e810c19729de860ea"`
	// Email адрес электронной почты пользователя.
	Email string `json:"email" bson:"email" example:"newuser@example.com"`
}

// RefreshToken представляет собой запись refresh токена в базе данных.
type RefreshToken struct {
	// ID записи токена.
	ID string `bson:"_id,omitempty"`
	// UserID идентификатор пользователя, которому принадлежит токен.
	UserID string `bson:"user_id"`
	// Token строковое значение токена.
	Token string `bson:"token"`
	// ExpiresAt время истечения срока действия токена.
	ExpiresAt time.Time `bson:"expires_at"`
	// CreatedAt время создания токена.
	CreatedAt time.Time `bson:"created_at"`
}

// Claims содержит утверждения JWT для access токена.
type Claims struct {
	// UserID идентификатор пользователя в токене.
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// RegisterRequest данные для регистрации нового пользователя.
type RegisterRequest struct {
	// Email адрес для регистрации.
	// Required: true
	Email string `json:"email" example:"newuser@example.com"`
	// Пароль для доступа к аккаунту.
	// Required: true
	Password string `json:"password" example:"SuperSecretPass123"`
}

// LoginRequest данные для входа в систему.
type LoginRequest struct {
	// Email адрес зарегистрированного пользователя.
	// Required: true
	Email string `json:"email" example:"newuser@example.com"`
	// Пароль пользователя.
	// Required: true
	Password string `json:"password" example:"SuperSecretPass123"`
}

// RefreshRequest данные для обновления пары токенов.
type RefreshRequest struct {
	// RefreshToken токен, полученный при предыдущем входе или обновлении.
	// Required: true
	RefreshToken string `json:"refresh_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

// TokenResponse ответ сервера при успешной аутентификации или обновлении токенов.
type TokenResponse struct {
	// AccessToken токен для авторизации защищенных запросов.
	AccessToken string `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	// RefreshToken токен для получения новой пары access/refresh.
	RefreshToken string `json:"refresh_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	// ExpiresIn время жизни access токена в секундах.
	ExpiresIn int64 `json:"expires_in" example:"3600"`
}

// ErrorResponse стандартный формат ответа об ошибке.
type ErrorResponse struct {
	// Error описание произошедшей ошибки.
	Error string `json:"error" example:"описание ошибки..."`
}
