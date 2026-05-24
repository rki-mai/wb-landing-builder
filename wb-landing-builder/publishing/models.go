package publishing

import "time"

// PublicationStatus — состояние жизненного цикла публикации.
type PublicationStatus string

const (
	// StatusFinished — публикация завершена (синхронный сценарий).
	StatusFinished PublicationStatus = "FINISHED"
)

// Publication описывает опубликованный снимок лендинга в объектном хранилище.
type Publication struct {
	ID         string            `json:"id" bson:"_id"`
	ProjectID  string            `json:"project_id" bson:"project_id"`
	Version    int               `json:"version" bson:"version"`
	AssetsPath string            `json:"assets_path" bson:"assets_path"`
	Status     PublicationStatus `json:"status" bson:"status"`
	CreatedAt  time.Time         `json:"created_at" bson:"created_at"`
}

// CreatePublicationRequest — тело запроса POST /api/v1/publications.
type CreatePublicationRequest struct {
	ProjectID string `json:"project_id" example:"my-project"`
}

// ErrorResponse — стандартный формат ответа об ошибке.
type ErrorResponse struct {
	Error string `json:"error" example:"описание ошибки..."`
}
