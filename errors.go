package garden

import (
	"encoding/json"
	"errors"
	"net/http"
)

type errType string

const (
	unrecoverableErrType      = "UnrecoverableError"
	serviceUnavailableErrType = "ServiceUnavailableError"
	containerNotFoundErrType  = "ContainerNotFoundError"
	processNotFoundErrType    = "ProcessNotFoundError"
	executableNotFoundError   = "ExecutableNotFoundError"
)

type Error struct {
	Err error
}

func NewError(err string) *Error {
	return &Error{Err: errors.New(err)}
}

type marshalledError struct {
	Type      errType
	Message   string
	Handle    string
	ProcessID string
	Binary    string
}

func (m Error) Error() string {
	return m.Err.Error()
}

func (m Error) StatusCode() int {
	switch m.Err.(type) {
	case ContainerNotFoundError:
		return http.StatusNotFound
	case ProcessNotFoundError:
		return http.StatusNotFound
	}

	return http.StatusInternalServerError
}

func (m Error) MarshalJSON() ([]byte, error) {
	var errorType errType
	handle := ""
	processID := ""
	switch err := m.Err.(type) {
	case ContainerNotFoundError:
		errorType = containerNotFoundErrType
		handle = err.Handle
	case ProcessNotFoundError:
		errorType = processNotFoundErrType
		processID = err.ProcessID
	case ExecutableNotFoundError:
		errorType = executableNotFoundError
	case ServiceUnavailableError:
		errorType = serviceUnavailableErrType
	case UnrecoverableError:
		errorType = unrecoverableErrType
	}

	return json.Marshal(marshalledError{
		Type:      errorType,
		Message:   m.Err.Error(),
		Handle:    handle,
		ProcessID: processID,
	})
}

func (m *Error) UnmarshalJSON(data []byte) error {
	var result marshalledError

	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}

	switch result.Type {
	case unrecoverableErrType:
		m.Err = UnrecoverableError{result.Message}
	case serviceUnavailableErrType:
		m.Err = ServiceUnavailableError{result.Message}
	case containerNotFoundErrType:
		m.Err = ContainerNotFoundError{result.Handle}
	case processNotFoundErrType:
		m.Err = ProcessNotFoundError{ProcessID: result.ProcessID}
	case executableNotFoundError:
		m.Err = ExecutableNotFoundError{Message: result.Message}
	default:
		m.Err = errors.New(result.Message)
	}

	return nil
}

func NewUnrecoverableError(symptom string) error {
	return UnrecoverableError{
		Symptom: symptom,
	}
}

type UnrecoverableError struct {
	Symptom string
}

func (err UnrecoverableError) Error() string {
	return err.Symptom
}

type ContainerNotFoundError struct {
	Handle string
}

func (err ContainerNotFoundError) Error() string {
	return "unknown handle: " + err.Handle
}

func NewServiceUnavailableError(cause string) error {
	return ServiceUnavailableError{
		Cause: cause,
	}
}

type ServiceUnavailableError struct {
	Cause string
}

func (err ServiceUnavailableError) Error() string {
	return err.Cause
}

type ProcessNotFoundError struct {
	ProcessID string
}

func (err ProcessNotFoundError) Error() string {
	return "unknown process: " + err.ProcessID
}

type ExecutableNotFoundError struct {
	Message string
}

func (err ExecutableNotFoundError) Error() string {
	return err.Message
}
