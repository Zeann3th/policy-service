package core

type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func (e *AppError) Is(target error) bool {
	targetError, ok := target.(*AppError)
	return ok && e.Code == targetError.Code
}

func (e *AppError) WithCause(cause error) *AppError {
	clone := *e
	clone.Cause = cause
	return &clone
}

var (
	ErrBadRequest = &AppError{Code: "BAD_REQUEST", Message: "bad request"}
	ErrNotFound   = &AppError{Code: "NOT_FOUND", Message: "not found"}
	ErrInternal   = &AppError{Code: "INTERNAL_ERROR", Message: "internal server error"}
)
