package storage

import (
	"go.mongodb.org/mongo-driver/bson"
)

// OperationType определяет тип операции мутации.
// swagger:enum OperationType
type OperationType string

const (
	// OperationCreate создает новый элемент или блок.
	OperationCreate OperationType = "create"
	// OperationUpdate обновляет существующий элемент.
	OperationUpdate OperationType = "update"
	// OperationDelete удаляет элемент.
	OperationDelete OperationType = "delete"
)

// Draft представляет собой черновик страницы, хранящий историю изменений.
// @Description Черновик содержит сериализованные мутации.
type Draft struct {
	// Mutations - байтовое представление списка мутаций (BSON/JSON).
	Mutations []byte `json:"mutations" example:"[]"`
}

// Mutation описывает единичное изменение в структуре страницы.
// @Description Объект мутации, применяемый к черновику.
type Mutation struct {
	// Operation тип операции: create, update или delete.
	// Required: true
	// Enum: [create, update, delete]
	Operation OperationType `json:"operation" example:"create"`

	// Data полезные данные мутации. Структура зависит от типа операции.
	// Для 'create' может содержать тип элемента и его свойства.
	// Для 'update' содержит ID элемента и новые значения.
	// Для 'delete' содержит ID элемента.
	Data bson.M `json:"data" swaggertype:"object" example:"{\"id\":\"header-1\",\"content\":\"Hello World\"}"`
}

// Project представляет пользовательский проект.
// @Description Проект пользователя.
type Project struct {
	// ID уникальный идентификатор проекта.
	ID string `json:"id" example:"project-123"`

	// Name отображаемое имя проекта.
	Name string `json:"name" example:"Landing Page"`
}

// ErrorResponse стандартный формат ответа об ошибке.
type ErrorResponse struct {
	// Error описание произошедшей ошибки.
	Error string `json:"error" example:"описание ошибки..."`
}
