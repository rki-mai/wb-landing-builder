package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Block struct {
	Type  string                 `json:"type" bson:"type"`
	Props map[string]interface{} `json:"props" bson:"props"` // ← было interface{}, стало map[string]interface{}
}

type Landing struct {
	ID        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Title     string             `json:"title" bson:"title"`
	Blocks    []Block            `json:"blocks" bson:"blocks"`
	Published bool               `json:"published" bson:"published"`
	PublicUrl string             `json:"publicUrl,omitempty" bson:"publicUrl"`
}
