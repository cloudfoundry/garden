package transport

import (
	"encoding/json"
	"io"

	"code.google.com/p/goprotobuf/proto"
)

func WriteMessage(writer io.Writer, req proto.Message) error {
	return json.NewEncoder(writer).Encode(req)
}
