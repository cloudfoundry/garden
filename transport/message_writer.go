package transport

import (
	"encoding/json"
	"io"
)

func WriteMessage(writer io.Writer, req interface{}) error {
	return json.NewEncoder(writer).Encode(req)
}
