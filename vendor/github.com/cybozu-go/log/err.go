package log

import "errors"

var (
	// ErrTooLarge is returned for too large log.
	ErrTooLarge = errors.New("Too large log")

	// ErrInvalidKey is returned when fields contain invalid key.
	ErrInvalidKey = errors.New("Invalid key")

	// ErrInvalidData is returned when fields contain invalid data.
	ErrInvalidData = errors.New("Invalid data type")
)
