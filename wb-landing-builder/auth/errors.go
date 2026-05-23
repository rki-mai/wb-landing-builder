package auth

import "errors"

var (
	ErrUserAlreadyExists       = errors.New("user already exists")
	ErrUserNotFound            = errors.New("user not found")
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrInvalidRefreshToken     = errors.New("invalid refresh token")
	ErrRefreshTokenExpired     = errors.New("refresh token expired")
	ErrInvalidToken            = errors.New("invalid token")
	ErrInvalidClaims           = errors.New("invalid token claims")
	ErrUnexpectedSigningMethod = errors.New("unexpected signing method")
)
