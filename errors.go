package garden

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type errType string

const (
	unrecoverableErrType      = "UnrecoverableError"
	serviceUnavailableErrType = "ServiceUnavailableError"
	containerNotFoundErrType  = "ContainerNotFoundError"
)

type Error struct {
	Err error
}

func NewError(err string) *Error {
	return &Error{Err: errors.New(err)}
}

type marshalledError struct {
	Type    errType
	Message string
	Handle  string
}

func (m Error) Error() string {
	return m.Err.Error()
}

func (m Error) StatusCode() int {
	switch m.Err.(type) {
	case ContainerNotFoundError:
		return http.StatusNotFound
	}

	return http.StatusInternalServerError
}

func (m Error) MarshalJSON() ([]byte, error) {
	var errorType errType
	handle := ""
	switch err := m.Err.(type) {
	case ContainerNotFoundError:
		errorType = containerNotFoundErrType
		handle = err.Handle
	case ServiceUnavailableError:
		errorType = serviceUnavailableErrType
	case UnrecoverableError:
		errorType = unrecoverableErrType
	}

	return json.Marshal(marshalledError{errorType, m.Err.Error(), handle})
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
	return fmt.Sprintf("unknown handle: %s", err.Handle)
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
