package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rki-mai/wb-landing-builder/httputil"
)

type Handler struct {
	service *AuthService
}

func NewAuthHandler(svc *AuthService) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes регистрирует маршруты аутентификации.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, middleware func(http.Handler) http.Handler) {
	mux.HandleFunc("POST /api/v1/auth/register", h.register)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.refresh)

	mux.Handle("GET /api/v1/auth/me", middleware(http.HandlerFunc(h.me)))
}

// Register регистрирует нового пользователя.
// @Summary Регистрация
// @Description Создает нового пользователя и возвращает его ID и email.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Данные для регистрации"
// @Success 201 {object} Me "ID и email созданного пользователя"
// @Failure 400 {object} ErrorResponse "Ошибка валидации или формата"
// @Failure 409 {object} ErrorResponse "Пользователь уже существует"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/auth/register [post]
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "email and password required")
		return
	}

	user, err := h.service.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			httputil.WriteJSONError(w, http.StatusConflict, err.Error())
			return
		}
		httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to register")
		return
	}

	httputil.WriteJSONResponse(w, http.StatusCreated, map[string]string{
		"id":    user.ID,
		"email": user.Email,
	})
}

// Login выполняет вход в систему.
// @Summary Вход
// @Description Аутентифицирует пользователя и возвращает токены доступа.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Данные для входа"
// @Success 200 {object} TokenResponse "Токены доступа"
// @Failure 400 {object} ErrorResponse "Неверный формат запроса"
// @Failure 401 {object} ErrorResponse "Неверные учетные данные"
// @Router /api/v1/auth/login [post]
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tokens, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			httputil.WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}
		httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to login: "+err.Error())
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, tokens)
}

// Refresh обновляет токены доступа.
// @Summary Обновление токенов
// @Description Обновляет пару токенов, используя refresh токен.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "Refresh токен"
// @Success 200 {object} TokenResponse "Новая пара токенов"
// @Failure 400 {object} ErrorResponse "Отсутствует refresh токен"
// @Failure 401 {object} ErrorResponse "Невалидный refresh токен"
// @Router /api/v1/auth/refresh [post]
func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		httputil.WriteJSONError(w, http.StatusBadRequest, "refresh_token required")
		return
	}

	tokens, err := h.service.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrInvalidRefreshToken) || errors.Is(err, ErrRefreshTokenExpired) {
			httputil.WriteJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}
		httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to refresh: "+err.Error())
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, tokens)
}

// Me получает информацию о текущем пользователе.
// @Summary Мой профиль
// @Description Возвращает ID и email авторизованного пользователя.
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} Me "ID и email пользователя"
// @Failure 401 {object} ErrorResponse "Не авторизован или пользователь не найден"
// @Router /api/v1/auth/me [get]
func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.service.GetUserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			httputil.WriteJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		httputil.WriteJSONError(w, http.StatusInternalServerError, "failed to get user: "+err.Error())
		return
	}

	httputil.WriteJSONResponse(w, http.StatusOK, map[string]string{
		"id":    user.ID,
		"email": user.Email,
	})
}
