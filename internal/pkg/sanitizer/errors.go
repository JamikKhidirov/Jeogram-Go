package sanitizer

import "errors"

var (
	ErrTextTooLong = errors.New("text exceeds maximum length of 4096 characters")
	ErrEmptyName   = errors.New("name cannot be empty")
	ErrNameTooLong = errors.New("name exceeds maximum length of 100 characters")
)
