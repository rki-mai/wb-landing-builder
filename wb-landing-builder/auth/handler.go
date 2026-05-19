package auth

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type Handler struct {
	service *AuthService
}

func NewAuthHandler(svc *AuthService) *Handler {
	return &Handler{service: svc}
}

func (h *Handler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "register",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/register",
		Summary:     "Register",
		Tags:        []string{"Auth"},
		Security:    []map[string][]string{},
	}, h.register)

	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/login",
		Summary:     "Login",
		Tags:        []string{"Auth"},
		Security:    []map[string][]string{},
	}, h.login)

	huma.Register(api, huma.Operation{
		OperationID: "refresh",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/refresh",
		Summary:     "Refresh Tokens",
		Tags:        []string{"Auth"},
		Security:    []map[string][]string{},
	}, h.refresh)

	huma.Register(api, huma.Operation{
		OperationID: "me",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/me",
		Summary:     "My Profile",
		Tags:        []string{"Auth"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
	}, h.me)
}

func (h *Handler) register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	if req.Body.Email == "" || req.Body.Password == "" {
		return nil, huma.Error400BadRequest("email and password required")
	}

	user, err := h.service.Register(ctx, req.Body.Email, req.Body.Password)
	if err != nil {
		if err.Error() == "user already exists" {
			return nil, huma.Error409Conflict(err.Error())
		}
		return nil, huma.Error500InternalServerError("failed to register")
	}

	return &RegisterResponse{
		Body: struct {
			ID    string `json:"id" example:"507f1f77bcf86cd799439011" doc:"User ID"`
			Email string `json:"email" example:"user@example.com" doc:"User email"`
		}{
			ID:    user.ID,
			Email: user.Email,
		},
	}, nil
}

func (h *Handler) login(ctx context.Context, req *LoginRequest) (*TokenResponse, error) {
	tokens, err := h.service.Login(ctx, req.Body.Email, req.Body.Password)
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}

	return tokens, nil
}

func (h *Handler) refresh(ctx context.Context, req *RefreshRequest) (*TokenResponse, error) {
	if req.Body.RefreshToken == "" {
		return nil, huma.Error400BadRequest("refresh_token required")
	}

	tokens, err := h.service.Refresh(ctx, req.Body.RefreshToken)
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid refresh token")
	}

	return tokens, nil
}

func (h *Handler) me(ctx context.Context, req *struct{}) (*MeResponse, error) {
	userID, ok := ctx.Value(UserContextKey).(string)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	user, err := h.service.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return nil, huma.Error401Unauthorized("user not found")
	}

	return &MeResponse{
		Body: struct {
			ID    string `json:"id" example:"507f1f77bcf86cd799439011" doc:"User ID"`
			Email string `json:"email" example:"user@example.com" doc:"User email"`
		}{
			ID:    user.ID,
			Email: user.Email,
		},
	}, nil
}
