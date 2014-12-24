package transport

import (
	"encoding/json"
	"io"

	"github.com/gogo/protobuf/proto"
)

func WriteMessage(writer io.Writer, req proto.Message) error {
	return json.NewEncoder(writer).Encode(req)
}
