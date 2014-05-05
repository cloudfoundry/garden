package connection

import (
	"code.google.com/p/goprotobuf/proto"
	protocol "github.com/cloudfoundry-incubator/garden/protocol"
)

type protobufStreamWriter struct {
	conn *connection
}

func NewProtobufStreamWriter(conn *connection) protobufStreamWriter {
	return protobufStreamWriter{
		conn: conn,
	}
}

func (w protobufStreamWriter) Write(buff []byte) (int, error) {
	chunk := protocol.StreamChunk{
		Content: buff,
	}
	err := w.conn.sendMessage(&chunk)
	return len(buff), err
}

func (w protobufStreamWriter) Close() error {
	return w.conn.sendMessage(&protocol.StreamChunk{
		Content: []byte{},
		EOF:     proto.Bool(true),
	})
}
