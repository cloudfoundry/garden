package transport

import (
	"fmt"
	"io"

	"code.google.com/p/goprotobuf/proto"
	protocol "github.com/cloudfoundry-incubator/garden/protocol"
)

func WriteMessage(writer io.Writer, req proto.Message) error {
	request, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	msg := &protocol.Message{
		Type:    protocol.TypeForMessage(req).Enum(),
		Payload: request,
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = writer.Write(
		[]byte(
			fmt.Sprintf(
				"%d\n%s",
				len(data),
				data,
			),
		),
	)

	return err
}
