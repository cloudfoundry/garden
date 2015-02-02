package garden

import (
	"bytes"
	"encoding/json"

	"github.com/gogo/protobuf/proto"
)

func Messages(msgs ...proto.Message) *bytes.Buffer {
	buf := bytes.NewBuffer([]byte{})

	encoder := json.NewEncoder(buf)

	for _, msg := range msgs {
		payload, err := json.Marshal(msg)
		if err != nil {
			panic(err.Error())
		}

		err = encoder.Encode(&Message{
			Type:    TypeForMessage(msg).Enum(),
			Payload: payload,
		})
		if err != nil {
			panic("failed to encode message: " + err.Error())
		}
	}

	return buf
}

func TypeForMessage(msg proto.Message) Message_Type {
	switch msg.(type) {
	case *ErrorResponse:
		return Message_Error

	case *LimitMemoryRequest, *LimitMemoryResponse:
		return Message_LimitMemory
	case *LimitDiskRequest, *LimitDiskResponse:
		return Message_LimitDisk
	case *LimitBandwidthRequest, *LimitBandwidthResponse:
		return Message_LimitBandwidth
	case *LimitCpuRequest, *LimitCpuResponse:
		return Message_LimitCpu

	}

	panic("unknown message type")
}

func RequestMessageForType(t Message_Type) proto.Message {
	switch t {

	case Message_LimitMemory:
		return &LimitMemoryRequest{}
	case Message_LimitDisk:
		return &LimitDiskRequest{}
	case Message_LimitBandwidth:
		return &LimitBandwidthRequest{}
	case Message_LimitCpu:
		return &LimitCpuRequest{}

	}

	panic("unknown message type")
}

func ResponseMessageForType(t Message_Type) proto.Message {
	switch t {

	case Message_LimitMemory:
		return &LimitMemoryResponse{}
	case Message_LimitDisk:
		return &LimitDiskResponse{}
	case Message_LimitBandwidth:
		return &LimitBandwidthResponse{}
	case Message_LimitCpu:
		return &LimitCpuResponse{}

	}

	panic("unknown message type")
}
