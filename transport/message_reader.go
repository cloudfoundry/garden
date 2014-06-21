package transport

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"code.google.com/p/gogoprotobuf/proto"

	protocol "github.com/cloudfoundry-incubator/garden/protocol"
)

type WardenError struct {
	Message   string
	Data      string
	Backtrace []string
}

func (e *WardenError) Error() string {
	return e.Message
}

type TypeMismatchError struct {
	Expected protocol.Message_Type
	Received protocol.Message_Type
}

func (e *TypeMismatchError) Error() string {
	return fmt.Sprintf(
		"expected message type %s, got %s\n",
		e.Expected,
		e.Received,
	)
}

func ReadMessage(reader io.Reader, response proto.Message) error {
	payload, err := readPayload(reader)
	if err != nil {
		return err
	}

	message := &protocol.Message{}
	err = proto.Unmarshal(payload, message)
	if err != nil {
		return err
	}

	// error response from server
	if message.GetType() == protocol.Message_Type(1) {
		errorResponse := &protocol.ErrorResponse{}
		err = proto.Unmarshal(message.Payload, errorResponse)
		if err != nil {
			return errors.New("error unmarshalling error!")
		}

		return &WardenError{
			Message:   errorResponse.GetMessage(),
			Data:      errorResponse.GetData(),
			Backtrace: errorResponse.GetBacktrace(),
		}
	}

	responseType := protocol.TypeForMessage(response)
	if message.GetType() != responseType {
		return &TypeMismatchError{
			Expected: responseType,
			Received: message.GetType(),
		}
	}

	return proto.Unmarshal(message.GetPayload(), response)
}

func ReadRequest(reader io.Reader) (proto.Message, error) {
	payload, err := readPayload(reader)
	if err != nil {
		return nil, err
	}

	message := &protocol.Message{}
	err = proto.Unmarshal(payload, message)
	if err != nil {
		return nil, err
	}

	request := protocol.RequestMessageForType(message.GetType())

	err = proto.Unmarshal(message.GetPayload(), request)
	if err != nil {
		return nil, err
	}

	return request, nil
}

func readPayload(reader io.Reader) ([]byte, error) {
	msgLen, err := readHeader(reader)
	if err != nil {
		return nil, err
	}

	return readNBytes(int(msgLen), reader)
}

func readHeader(reader io.Reader) (int, error) {
	chr := make([]byte, 1)

	length := 0
	for {
		_, err := reader.Read(chr)
		if err != nil {
			return 0, err
		}

		if chr[0] == '\n' {
			break
		}

		num, err := strconv.Atoi(string(chr))
		if err != nil {
			return 0, err
		}

		length *= 10
		length += num
	}

	return length, nil
}

func readNBytes(payloadLen int, reader io.Reader) ([]byte, error) {
	payload := make([]byte, payloadLen)

	for readCount := 0; readCount < payloadLen; {
		n, err := reader.Read(payload[readCount:])
		if n == 0 && err != nil {
			return nil, err
		}

		readCount += n
	}

	return payload, nil
}
