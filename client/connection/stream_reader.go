package connection

import (
	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"io"
)

type protobufStreamReader struct {
	conn   *connection
	buffer []byte
}

func NewProtobufStreamReader(conn *connection) *protobufStreamReader {
	return &protobufStreamReader{
		conn: conn,
	}
}

func (r *protobufStreamReader) Read(buff []byte) (int, error) {
	if len(r.buffer) > 0 {
		n := copy(buff, r.buffer)
		r.buffer = r.buffer[n:]
		return n, nil
	}

	response := &protocol.StreamChunk{}
	err := r.conn.readResponse(response)
	if err != nil {
		return 0, err
	}

	if response.GetEOF() {
		return 0, io.EOF
	}

	r.buffer = response.Content
	return r.Read(buff)
}
