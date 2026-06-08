package publishing

import (
	"errors"
	"net/http"

	"github.com/rki-mai/wb-landing-builder/auth"
	"github.com/rki-mai/wb-landing-builder/httputil"
	"github.com/rki-mai/wb-landing-builder/storage"
)

func userIDFromRequest(r *http.Request) (string, bool) {
	userID, ok := r.Context().Value(auth.UserIDKey).(string)
	return userID, ok && userID != ""
}

func writePublicationError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, storage.ErrForbidden) {
		httputil.WriteJSONError(w, http.StatusForbidden, err.Error())
		return
	}
	if errors.Is(err, storage.ErrProjectNotFound) || errors.Is(err, storage.ErrDraftNotFound) {
		httputil.WriteJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, ErrPublicationNotFound) {
		httputil.WriteJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	httputil.WriteJSONError(w, http.StatusInternalServerError, err.Error())
}
