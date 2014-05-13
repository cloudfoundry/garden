package transport

import (
	"fmt"
	"net"

	"code.google.com/p/goprotobuf/proto"
	protocol "github.com/cloudfoundry-incubator/garden/protocol"
)

func WriteMessage(conn net.Conn, req proto.Message) error {
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

	_, err = conn.Write(
		[]byte(
			fmt.Sprintf(
				"%d\r\n%s\r\n",
				len(data),
				data,
			),
		),
	)

	return err
}
