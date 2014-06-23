package warden

import (
	"bytes"
	"encoding/json"

	"code.google.com/p/gogoprotobuf/proto"
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

	case *CreateRequest, *CreateResponse:
		return Message_Create
	case *StopRequest, *StopResponse:
		return Message_Stop
	case *DestroyRequest, *DestroyResponse:
		return Message_Destroy
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

	case *PingRequest, *PingResponse:
		return Message_Ping
	case *ListRequest, *ListResponse:
		return Message_List
	case *CapacityRequest, *CapacityResponse:
		return Message_Capacity
	}

	panic("unknown message type")
}

func RequestMessageForType(t Message_Type) proto.Message {
	switch t {
	case Message_Create:
		return &CreateRequest{}
	case Message_Stop:
		return &StopRequest{}
	case Message_Destroy:
		return &DestroyRequest{}
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

	case Message_Ping:
		return &PingRequest{}
	case Message_List:
		return &ListRequest{}
	case Message_Capacity:
		return &CapacityRequest{}
	}

	panic("unknown message type")
}

func ResponseMessageForType(t Message_Type) proto.Message {
	switch t {
	case Message_Create:
		return &CreateResponse{}
	case Message_Stop:
		return &StopResponse{}
	case Message_Destroy:
		return &DestroyResponse{}
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

	case Message_Ping:
		return &PingResponse{}
	case Message_List:
		return &ListResponse{}
	case Message_Capacity:
		return &CapacityResponse{}
	}

	panic("unknown message type")
}
