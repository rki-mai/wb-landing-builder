package worker

import (
	"context"
	"log"

	"github.com/rki-mai/wb-landing-builder/publishing"
	"github.com/rki-mai/wb-landing-builder/publishing/utils"
)

// Worker обрабатывает задачи на рендер из очереди.
type Worker struct {
	consumer utils.Consumer
	service  *publishing.PublicationService
}

// New создаёт worker публикаций.
func New(consumer utils.Consumer, service *publishing.PublicationService) *Worker {
	return &Worker{
		consumer: consumer,
		service:  service,
	}
}

// Run запускает цикл обработки задач до отмены контекста.
func (w *Worker) Run(ctx context.Context) error {
	log.Println("Publication worker started")
	return w.consumer.Consume(ctx, w.service.ProcessPublication)
}
