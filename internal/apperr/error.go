package apperr

import "fmt"

type ExitStatus int

const (
	ExitSuccess ExitStatus = iota
	ExitGeneralRuntimeError
	ExitInvalidArguments
	ExitStoreNotInitialized
	ExitNotFound
)

type Error struct {
	Code    string
	Message string
	Status  ExitStatus
}

func (e *Error) Error() string {
	return e.Message
}

func (s ExitStatus) Code() int {
	return int(s)
}

func InvalidArguments(message string, args ...any) *Error {
	return newError("INVALID_ARGUMENTS", ExitInvalidArguments, message, args...)
}

func HomeDirectoryUnavailable() *Error {
	return newError("HOME_DIRECTORY_UNAVAILABLE", ExitGeneralRuntimeError, "Could not determine a home directory for the default store path.")
}

func StoreAlreadyExists(path string) *Error {
	return newError("STORE_ALREADY_EXISTS", ExitGeneralRuntimeError, "A store already exists at %s. Re-run with `--force` to reinitialize it.", path)
}

func StoreNotFound(path string) *Error {
	return newError("STORE_NOT_FOUND", ExitStoreNotInitialized, "No store at %s. Run `aascribe init`.", path)
}

func NotImplemented(command string) *Error {
	return newError("NOT_IMPLEMENTED", ExitGeneralRuntimeError, "The command `%s` is not implemented yet.", command)
}

func IOError(message string, args ...any) *Error {
	return newError("IO_ERROR", ExitGeneralRuntimeError, message, args...)
}

func Serialization(message string, args ...any) *Error {
	return newError("SERIALIZATION_ERROR", ExitGeneralRuntimeError, message, args...)
}

func newError(code string, status ExitStatus, message string, args ...any) *Error {
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(message, args...),
		Status:  status,
	}
}
