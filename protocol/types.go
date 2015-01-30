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

	case *StopRequest, *StopResponse:
		return Message_Stop
	case *InfoRequest, *InfoResponse:
		return Message_Info

	case *NetInRequest, *NetInResponse:
		return Message_NetIn
	case *NetOutRequest, *NetOutResponse:
		return Message_NetOut

	case *StreamInRequest, *StreamInResponse:
		return Message_StreamIn
	case *StreamOutRequest, *StreamOutResponse:
		return Message_StreamOut

	case *LimitMemoryRequest, *LimitMemoryResponse:
		return Message_LimitMemory
	case *LimitDiskRequest, *LimitDiskResponse:
		return Message_LimitDisk
	case *LimitBandwidthRequest, *LimitBandwidthResponse:
		return Message_LimitBandwidth
	case *LimitCpuRequest, *LimitCpuResponse:
		return Message_LimitCpu

	case *RunRequest:
		return Message_Run
	case *AttachRequest:
		return Message_Attach
	case *ProcessPayload:
		return Message_ProcessPayload

	}

	panic("unknown message type")
}

func RequestMessageForType(t Message_Type) proto.Message {
	switch t {
	case Message_Stop:
		return &StopRequest{}
	case Message_Info:
		return &InfoRequest{}

	case Message_NetIn:
		return &NetInRequest{}
	case Message_NetOut:
		return &NetOutRequest{}

	case Message_StreamIn:
		return &StreamInRequest{}
	case Message_StreamOut:
		return &StreamOutRequest{}

	case Message_LimitMemory:
		return &LimitMemoryRequest{}
	case Message_LimitDisk:
		return &LimitDiskRequest{}
	case Message_LimitBandwidth:
		return &LimitBandwidthRequest{}
	case Message_LimitCpu:
		return &LimitCpuRequest{}

	case Message_Run:
		return &RunRequest{}
	case Message_Attach:
		return &AttachRequest{}

	}

	panic("unknown message type")
}

func ResponseMessageForType(t Message_Type) proto.Message {
	switch t {
	case Message_Stop:
		return &StopResponse{}
	case Message_Info:
		return &InfoResponse{}
	case Message_NetIn:
		return &NetInResponse{}
	case Message_NetOut:
		return &NetOutResponse{}

	case Message_StreamIn:
		return &StreamInResponse{}
	case Message_StreamOut:
		return &StreamOutResponse{}

	case Message_LimitMemory:
		return &LimitMemoryResponse{}
	case Message_LimitDisk:
		return &LimitDiskResponse{}
	case Message_LimitBandwidth:
		return &LimitBandwidthResponse{}
	case Message_LimitCpu:
		return &LimitCpuResponse{}

	case Message_Run, Message_Attach:
		return &ProcessPayload{}

	}

	panic("unknown message type")
}
