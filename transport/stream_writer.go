package transport

import (
	"net"

	"code.google.com/p/goprotobuf/proto"
	protocol "github.com/cloudfoundry-incubator/garden/protocol"
)

type protobufStreamWriter struct {
	conn net.Conn
}

func NewProtobufStreamWriter(conn net.Conn) protobufStreamWriter {
	return protobufStreamWriter{
		conn: conn,
	}
}

func (w protobufStreamWriter) Write(buff []byte) (int, error) {
	_, err := protocol.Messages(&protocol.StreamChunk{
		Content: buff,
	}).WriteTo(w.conn)
	if err != nil {
		return 0, err
	}

	return len(buff), nil
}

func (w protobufStreamWriter) Close() error {
	_, err := protocol.Messages(&protocol.StreamChunk{
		EOF: proto.Bool(true),
	}).WriteTo(w.conn)

	return err
}
