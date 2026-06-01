package utils

import "context"

// PublishTask — задача на рендер и загрузку публикации.
type PublishTask struct {
	PublicationID string `json:"publication_id"`
	ProjectID     string `json:"project_id"`
	UserID        string `json:"user_id"`
}

// Publisher отправляет задачи на рендер в очередь.
type Publisher interface {
	Publish(ctx context.Context, task PublishTask) error
	Close() error
}

// Consumer читает задачи из очереди и передаёт их обработчику.
type Consumer interface {
	Consume(ctx context.Context, handler func(ctx context.Context, task PublishTask) error) error
	Close() error
}

// Queue объединяет Publisher и Consumer для одного подключения к брокеру.
type Queue interface {
	Publisher
	Consumer
}
