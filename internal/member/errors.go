package member

import "errors"

var (
	ErrNotFound         = errors.New("member not found")
	ErrDuplicateStudent = errors.New("a member with this student ID already exists")
	ErrInvalidInput     = errors.New("invalid input")
	ErrInvalidRole      = errors.New("invalid club role")
)
