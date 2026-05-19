package storage

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
	Mutations string `json:"mutations" example:"[{\"_id\":\"6a0c80f11d173e69d96dfe96\",\"created_at\":\"2026-05-19T15:25:37.638Z\",\"deleted\":false,\"element\":\"text\",\"id\":\"lb-test\",\"index\":0,\"parentId\":\"lb-1\",\"project_id\":\"1\",\"styles\":{\"color\":\"#000000\",\"fontSize\":\"24px\"},\"value\":\"Новый заголовок\",\"version\":1}]" doc:"List of mutations in the draft"`
}

type Mutation struct {
	Operation OperationType `json:"operation" example:"create" doc:"Type of operation (create, update, delete)"`
	Data      bson.M        `json:"data" doc:"Data for the operation"`
}

type ErrorResponse struct {
	Body struct {
		Error string `json:"error" example:"Invalid credentials" doc:"Error message"`
	}
}

type ApplyMutationRequest struct {
	Body struct {
		Mutations []Mutation `json:"mutations" required:"true" doc:"List of mutations to apply"`
	}
}

type ApplyMutationResponse struct {
	Body struct {
		Status  string `json:"status" example:"success" doc:"Operation status"`
		Version string `json:"version" example:"1" doc:"Current version after mutation"`
	}
}

type GetDraftResponse struct {
	Body Draft `json:"mutations" doc:"List of mutations in the draft"`
}

type applyMutationInput struct {
	ProjectID string `path:"project_id"`
	Body      Mutation
}

type getLatestDraftInput struct {
	ProjectID string `path:"project_id"`
}

type getDraftByVersionInput struct {
	ProjectID string `path:"project_id"`
	Version   int    `path:"version"`
}
