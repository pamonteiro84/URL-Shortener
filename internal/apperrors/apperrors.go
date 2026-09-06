package apperrors

type Kind int

const (
	NotFound Kind = iota
	AlreadyExists
	Internal
	Unauthorized
	Forbidden
)

type AppError struct {
	Kind Kind
	Err  error
}

func (e *AppError) Error() string {
	return e.Err.Error()
}

func (e *AppError) Unwrap() error {
	return e.Err
}
