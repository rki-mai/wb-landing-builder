package storage

import "errors"

var (
	ErrForbidden            = errors.New("access denied")
	ErrDraftNotFound        = errors.New("draft not found")
	ErrInvalidMutation      = errors.New("invalid mutation")
	ErrMutationNotFound     = errors.New("mutation not found")
	ErrProjectNotFound      = errors.New("project not found")
	ErrProjectAlreadyExists = errors.New("project already exists")
)
