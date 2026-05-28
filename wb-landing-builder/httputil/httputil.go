package httputil

import (
	"encoding/json"
	"net/http"
)

func WriteJSONError(w http.ResponseWriter, status int, message string) {
	WriteJSONResponse(w, status, map[string]string{"error": message})
}

func WriteJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	switch v := data.(type) {
	case []byte:
		// Уже сериализованный JSON (например, черновик из storage).
		if _, err := w.Write(v); err != nil {
			http.Error(w, "failed to write response", http.StatusInternalServerError)
		}
	default:
		if err := json.NewEncoder(w).Encode(data); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
	}
}
