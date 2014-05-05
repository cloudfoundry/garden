package transport

import (
	"bufio"
	"io"

	protocol "github.com/cloudfoundry-incubator/garden/protocol"
)

type protobufStreamReader struct {
	reader *bufio.Reader
	buffer []byte
}

func NewProtobufStreamReader(reader *bufio.Reader) *protobufStreamReader {
	return &protobufStreamReader{
		reader: reader,
	}
}

func (r *protobufStreamReader) Read(buff []byte) (int, error) {
	if len(r.buffer) > 0 {
		n := copy(buff, r.buffer)
		r.buffer = r.buffer[n:]
		return n, nil
	}

	response := &protocol.StreamChunk{}
	err := ReadMessage(r.reader, response)
	if err != nil {
		return 0, err
	}

	if response.GetEOF() {
		return 0, io.EOF
	}

	r.buffer = response.Content
	return r.Read(buff)
}
