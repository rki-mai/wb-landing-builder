package utils

import (
	"bytes"
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Draft — свёрнутый снимок страницы из storage (массив элементов после collapse).
type Draft struct {
	elements []bson.M
}

// ParseDraftSnapshot разбирает JSON-снимок черновика.
func ParseDraftSnapshot(data []byte) (*Draft, error) {
	return parseDraftSnapshot(data)
}

// JSON возвращает снимок в JSON для CLI и index.json в bundle.
func (d *Draft) JSON() ([]byte, error) {
	return json.Marshal(d.elements)
}

func parseDraftSnapshot(data []byte) (*Draft, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var snapshot struct {
			Elements []bson.M `json:"elements"`
		}
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return nil, fmt.Errorf("failed to parse draft snapshot: %w", err)
		}
		elements := snapshot.Elements
		if elements == nil {
			elements = []bson.M{}
		}
		return &Draft{elements: elements}, nil
	}

	// Обратная совместимость: голый массив элементов.
	var elements []bson.M
	if err := json.Unmarshal(data, &elements); err == nil {
		if elements == nil {
			elements = []bson.M{}
		}
		return &Draft{elements: elements}, nil
	}

	// Fallback: extended JSON (если источник когда-нибудь отдаст BSON-типы в тексте).
	var raw bson.A
	if err := bson.UnmarshalExtJSON(data, false, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse draft snapshot: %w", err)
	}

	elements = make([]bson.M, 0, len(raw))
	for _, item := range raw {
		m, err := normalizeToBSONM(item)
		if err != nil {
			return nil, fmt.Errorf("failed to parse draft snapshot: %w", err)
		}
		elements = append(elements, m)
	}

	return &Draft{elements: elements}, nil
}

func normalizeToBSONM(v interface{}) (bson.M, error) {
	switch doc := v.(type) {
	case bson.M:
		return doc, nil
	case map[string]interface{}:
		return bson.M(doc), nil
	case primitive.D:
		m := make(bson.M, len(doc))
		for _, elem := range doc {
			val, err := normalizeBSONValue(elem.Value)
			if err != nil {
				return nil, err
			}
			m[elem.Key] = val
		}
		return m, nil
	default:
		return nil, fmt.Errorf("failed to parse draft snapshot: unexpected document type %T", v)
	}
}

func normalizeBSONValue(v interface{}) (interface{}, error) {
	switch val := v.(type) {
	case bson.M, map[string]interface{}, primitive.D:
		return normalizeToBSONM(val)
	case bson.A:
		out := make(bson.A, len(val))
		for i, item := range val {
			normalized, err := normalizeBSONValue(item)
			if err != nil {
				return nil, err
			}
			out[i] = normalized
		}
		return out, nil
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, item := range val {
			normalized, err := normalizeBSONValue(item)
			if err != nil {
				return nil, err
			}
			out[i] = normalized
		}
		return out, nil
	default:
		return v, nil
	}
}
