package models

import (
	"go.mongodb.org/mongo-driver/bson"
)

type OperationType string

const (
	OperationCreate OperationType = "create"
	OperationUpdate OperationType = "update"
	OperationDelete OperationType = "delete"
)

type Draft struct {
	Mutations []byte
}

type Mutation struct {
	Operation OperationType `json:"operation"`
	Data      bson.M        `json:"data"`
}
