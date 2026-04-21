package registry

import (
	"encoding/json"
	"fmt"

	"github.com/rki-mai/wb-landing-builder/draft-component/service"
)

type ElementType string

const (
	Text      ElementType = "text"
	Image     ElementType = "image"
	Button    ElementType = "button"
	Link      ElementType = "link"
	Container ElementType = "container"
)

// --- Конкретные структуры данных для операций ---

type CreateMutation struct {
	Element  ElementType       `json:"element"`
	Attrs    map[string]string `json:"attrs"`
	Styles   map[string]string `json:"styles,omitempty"`
	ParentID string            `json:"parentId"`
	Index    int               `json:"index"`
}

type UpdateMutation struct {
	ID     string                 `json:"id"`
	Fields map[string]interface{} `json:"fields"`
}

type DeleteMutation struct {
	ID string `json:"id"`
}

type Mutation interface {
	Apply(svc service.DraftService, draftID string) error
}

func (m CreateMutation) Apply(svc service.DraftService, draftID string) error {
	return svc.CreateElement(draftID, m)
}

func (m UpdateMutation) Apply(svc service.DraftService, draftID string) error {
	return svc.UpdateElement(draftID, m)
}

func (m DeleteMutation) Apply(svc service.DraftService, draftID string) error {
	return svc.DeleteElement(draftID, m.ID)
}

var mutationRegistry = map[string]func() Mutation{
	"create": func() Mutation { return &CreateMutation{} },
	"update": func() Mutation { return &UpdateMutation{} },
	"delete": func() Mutation { return &DeleteMutation{} },
}

type rawRequest struct {
	Operation string          `json:"operation"`
	Data      json.RawMessage `json:"data"`
}

func ParseMutation(body []byte) (Mutation, error) {
	var req rawRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	factory, ok := mutationRegistry[req.Operation]
	if !ok {
		return nil, fmt.Errorf("unknown operation: %s", req.Operation)
	}

	mutation := factory()

	if err := json.Unmarshal(req.Data, mutation); err != nil {
		return nil, err
	}

	return mutation, nil
}
