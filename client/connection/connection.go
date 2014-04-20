package connection

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"

	"code.google.com/p/goprotobuf/proto"

	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/warden"
)

var DisconnectedError = errors.New("disconnected")

type Connection interface {
	Close()

	Disconnected() <-chan struct{}
	RoundTrip(request proto.Message, response proto.Message) (proto.Message, error)

	Create(spec warden.ContainerSpec) (*protocol.CreateResponse, error)
	List(properties warden.Properties) (*protocol.ListResponse, error)
	Destroy(handle string) (*protocol.DestroyResponse, error)

	Stop(handle string, background, kill bool) (*protocol.StopResponse, error)

	Info(handle string) (*protocol.InfoResponse, error)

	CopyIn(handle string, src, dst string) (*protocol.CopyInResponse, error)
	CopyOut(handle string, src, dst, owner string) (*protocol.CopyOutResponse, error)

	LimitBandwidth(handle string, limits warden.BandwidthLimits) (*protocol.LimitBandwidthResponse, error)
	LimitCPU(handle string, limits warden.CPULimits) (*protocol.LimitCpuResponse, error)
	LimitDisk(handle string, limits warden.DiskLimits) (*protocol.LimitDiskResponse, error)
	LimitMemory(handle string, limit warden.MemoryLimits) (*protocol.LimitMemoryResponse, error)

	Run(handle string, spec warden.ProcessSpec) (*protocol.ProcessPayload, <-chan *protocol.ProcessPayload, error)
	Attach(handle string, processID uint32) (<-chan *protocol.ProcessPayload, error)

	NetIn(handle string, hostPort, containerPort uint32) (*protocol.NetInResponse, error)
	NetOut(handle string, network string, port uint32) (*protocol.NetOutResponse, error)
}

type connection struct {
	messages chan *protocol.Message

	disconnected   chan struct{}
	disconnectOnce *sync.Once

	conn      net.Conn
	read      *bufio.Reader
	writeLock sync.Mutex
	readLock  sync.Mutex
}

type Info struct {
	Network string
	Addr    string
}

func (i *Info) ProvideConnection() (Connection, error) {
	return Connect(i.Network, i.Addr)
}

type WardenError struct {
	Message   string
	Data      string
	Backtrace []string
}

func (e *WardenError) Error() string {
	return e.Message
}

func Connect(network, addr string) (Connection, error) {
	conn, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}

	return New(conn), nil
}

func New(conn net.Conn) Connection {
	messages := make(chan *protocol.Message)

	connection := &connection{
		messages: messages,

		disconnected:   make(chan struct{}),
		disconnectOnce: &sync.Once{},

		conn: conn,
		read: bufio.NewReader(conn),
	}

	go connection.readMessages()

	return connection
}

func (c *connection) Close() {
	c.conn.Close()
}

func (c *connection) Disconnected() <-chan struct{} {
	return c.disconnected
}

func (c *connection) Create(spec warden.ContainerSpec) (*protocol.CreateResponse, error) {
	props := []*protocol.Property{}
	for key, val := range spec.Properties {
		props = append(props, &protocol.Property{
			Key:   proto.String(key),
			Value: proto.String(val),
		})
	}

	req := &protocol.CreateRequest{Properties: props}

	res, err := c.RoundTrip(req, &protocol.CreateResponse{})
	if err != nil {
		return nil, err
	}

	return res.(*protocol.CreateResponse), nil
}

func (c *connection) Stop(handle string, background, kill bool) (*protocol.StopResponse, error) {
	res, err := c.RoundTrip(
		&protocol.StopRequest{
			Handle:     proto.String(handle),
			Background: proto.Bool(background),
			Kill:       proto.Bool(kill),
		},
		&protocol.StopResponse{},
	)

	if err != nil {
		return nil, err
	}

	return res.(*protocol.StopResponse), nil
}

func (c *connection) Destroy(handle string) (*protocol.DestroyResponse, error) {
	res, err := c.RoundTrip(
		&protocol.DestroyRequest{Handle: proto.String(handle)},
		&protocol.DestroyResponse{},
	)

	if err != nil {
		return nil, err
	}

	return res.(*protocol.DestroyResponse), nil
}

func (c *connection) Run(handle string, spec warden.ProcessSpec) (*protocol.ProcessPayload, <-chan *protocol.ProcessPayload, error) {
	err := c.SendMessage(
		&protocol.RunRequest{
			Handle: proto.String(handle),
			Script: proto.String(spec.Script),
			Rlimits: &protocol.ResourceLimits{
				As:         spec.Limits.As,
				Core:       spec.Limits.Core,
				Cpu:        spec.Limits.Cpu,
				Data:       spec.Limits.Data,
				Fsize:      spec.Limits.Fsize,
				Locks:      spec.Limits.Locks,
				Memlock:    spec.Limits.Memlock,
				Msgqueue:   spec.Limits.Msgqueue,
				Nice:       spec.Limits.Nice,
				Nofile:     spec.Limits.Nofile,
				Nproc:      spec.Limits.Nproc,
				Rss:        spec.Limits.Rss,
				Rtprio:     spec.Limits.Rtprio,
				Sigpending: spec.Limits.Sigpending,
				Stack:      spec.Limits.Stack,
			},
			Env: convertEnvironmentVariables(spec.EnvironmentVariables),
		},
	)

	if err != nil {
		return nil, nil, err
	}

	responses := make(chan *protocol.ProcessPayload)

	resMsg, err := c.ReadResponse(&protocol.ProcessPayload{})
	if err != nil {
		return nil, nil, err
	}

	firstResponse := resMsg.(*protocol.ProcessPayload)

	go func() {
		for {
			resMsg, err := c.ReadResponse(&protocol.ProcessPayload{})
			if err != nil {
				close(responses)
				break
			}

			response := resMsg.(*protocol.ProcessPayload)

			responses <- response

			if response.ExitStatus != nil {
				close(responses)
				break
			}
		}
	}()

	return firstResponse, responses, nil
}

func (c *connection) Attach(handle string, processID uint32) (<-chan *protocol.ProcessPayload, error) {
	err := c.SendMessage(
		&protocol.AttachRequest{
			Handle:    proto.String(handle),
			ProcessId: proto.Uint32(processID),
		},
	)

	if err != nil {
		return nil, err
	}

	responses := make(chan *protocol.ProcessPayload)

	go func() {
		for {
			resMsg, err := c.ReadResponse(&protocol.ProcessPayload{})
			if err != nil {
				close(responses)
				break
			}

			response := resMsg.(*protocol.ProcessPayload)

			responses <- response

			if response.ExitStatus != nil {
				close(responses)
				break
			}
		}
	}()

	return responses, nil
}

func (c *connection) NetIn(handle string, hostPort, containerPort uint32) (*protocol.NetInResponse, error) {
	res, err := c.RoundTrip(
		&protocol.NetInRequest{
			Handle:        proto.String(handle),
			HostPort:      proto.Uint32(hostPort),
			ContainerPort: proto.Uint32(containerPort),
		},
		&protocol.NetInResponse{},
	)

	if err != nil {
		return nil, err
	}

	return res.(*protocol.NetInResponse), nil
}

func (c *connection) NetOut(handle string, network string, port uint32) (*protocol.NetOutResponse, error) {
	res, err := c.RoundTrip(
		&protocol.NetOutRequest{
			Handle:  proto.String(handle),
			Network: proto.String(network),
			Port:    proto.Uint32(port),
		},
		&protocol.NetOutResponse{},
	)

	if err != nil {
		return nil, err
	}

	return res.(*protocol.NetOutResponse), nil
}

func (c *connection) LimitMemory(handle string, limits warden.MemoryLimits) (*protocol.LimitMemoryResponse, error) {
	res, err := c.RoundTrip(
		&protocol.LimitMemoryRequest{
			Handle:       proto.String(handle),
			LimitInBytes: proto.Uint64(limits.LimitInBytes),
		},
		&protocol.LimitMemoryResponse{},
	)

	if err != nil {
		return nil, err
	}

	return res.(*protocol.LimitMemoryResponse), nil
}

func (c *connection) LimitBandwidth(handle string, limits warden.BandwidthLimits) (*protocol.LimitBandwidthResponse, error) {
	res, err := c.RoundTrip(
		&protocol.LimitBandwidthRequest{
			Handle: proto.String(handle),
			Rate:   proto.Uint64(limits.RateInBytesPerSecond),
			Burst:  proto.Uint64(limits.BurstRateInBytesPerSecond),
		},
		&protocol.LimitBandwidthResponse{},
	)

	if err != nil {
		return nil, err
	}

	return res.(*protocol.LimitBandwidthResponse), nil
}

func (c *connection) LimitCPU(handle string, limits warden.CPULimits) (*protocol.LimitCpuResponse, error) {
	res, err := c.RoundTrip(
		&protocol.LimitCpuRequest{
			Handle:        proto.String(handle),
			LimitInShares: proto.Uint64(limits.LimitInShares),
		},
		&protocol.LimitCpuResponse{},
	)

	if err != nil {
		return nil, err
	}

	return res.(*protocol.LimitCpuResponse), nil
}

func (c *connection) LimitDisk(handle string, limits warden.DiskLimits) (*protocol.LimitDiskResponse, error) {
	res, err := c.RoundTrip(
		&protocol.LimitDiskRequest{
			Handle: proto.String(handle),

			BlockLimit: proto.Uint64(limits.BlockLimit),
			Block:      proto.Uint64(limits.Block),
			BlockSoft:  proto.Uint64(limits.BlockSoft),
			BlockHard:  proto.Uint64(limits.BlockHard),

			InodeLimit: proto.Uint64(limits.InodeLimit),
			Inode:      proto.Uint64(limits.Inode),
			InodeSoft:  proto.Uint64(limits.InodeSoft),
			InodeHard:  proto.Uint64(limits.InodeHard),

			ByteLimit: proto.Uint64(limits.ByteLimit),
			Byte:      proto.Uint64(limits.Byte),
			ByteSoft:  proto.Uint64(limits.ByteSoft),
			ByteHard:  proto.Uint64(limits.ByteHard),
		},
		&protocol.LimitDiskResponse{},
	)

	if err != nil {
		return nil, err
	}

	return res.(*protocol.LimitDiskResponse), nil
}

func (c *connection) CopyIn(handle, src, dst string) (*protocol.CopyInResponse, error) {
	res, err := c.RoundTrip(
		&protocol.CopyInRequest{
			Handle:  proto.String(handle),
			SrcPath: proto.String(src),
			DstPath: proto.String(dst),
		},
		&protocol.CopyInResponse{},
	)

	if err != nil {
		return nil, err
	}

	return res.(*protocol.CopyInResponse), nil
}

func (c *connection) CopyOut(handle, src, dst, owner string) (*protocol.CopyOutResponse, error) {
	res, err := c.RoundTrip(
		&protocol.CopyOutRequest{
			Handle:  proto.String(handle),
			SrcPath: proto.String(src),
			DstPath: proto.String(dst),
			Owner:   proto.String(owner),
		},
		&protocol.CopyOutResponse{},
	)

	if err != nil {
		return nil, err
	}

	return res.(*protocol.CopyOutResponse), nil
}

func (c *connection) List(filterProperties warden.Properties) (*protocol.ListResponse, error) {
	props := []*protocol.Property{}
	for key, val := range filterProperties {
		props = append(props, &protocol.Property{
			Key:   proto.String(key),
			Value: proto.String(val),
		})
	}

	req := &protocol.ListRequest{Properties: props}

	res, err := c.RoundTrip(req, &protocol.ListResponse{})
	if err != nil {
		return nil, err
	}

	return res.(*protocol.ListResponse), nil
}

func (c *connection) Info(handle string) (*protocol.InfoResponse, error) {
	res, err := c.RoundTrip(&protocol.InfoRequest{
		Handle: proto.String(handle),
	}, &protocol.InfoResponse{})
	if err != nil {
		return nil, err
	}

	return res.(*protocol.InfoResponse), nil
}

func (c *connection) RoundTrip(request proto.Message, response proto.Message) (proto.Message, error) {
	err := c.SendMessage(request)
	if err != nil {
		return nil, err
	}

	resp, err := c.ReadResponse(response)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *connection) SendMessage(req proto.Message) error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	request, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	msg := &protocol.Message{
		Type:    protocol.TypeForMessage(req).Enum(),
		Payload: request,
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = c.conn.Write(
		[]byte(
			fmt.Sprintf(
				"%d\r\n%s\r\n",
				len(data),
				data,
			),
		),
	)

	if err != nil {
		c.notifyDisconnected()
		return err
	}

	return nil
}

func (c *connection) readMessages() {
	for {
		payload, err := c.readPayload()
		if err != nil {
			c.notifyDisconnected()
			close(c.messages)
			break
		}

		message := &protocol.Message{}
		err = proto.Unmarshal(payload, message)
		if err != nil {
			continue
		}

		c.messages <- message
	}
}

func (c *connection) notifyDisconnected() {
	c.disconnectOnce.Do(func() {
		close(c.disconnected)
	})
}

func (c *connection) ReadResponse(response proto.Message) (proto.Message, error) {
	message, ok := <-c.messages
	if !ok {
		return nil, DisconnectedError
	}

	if message.GetType() == protocol.Message_Error {
		errorResponse := &protocol.ErrorResponse{}
		err := proto.Unmarshal(message.Payload, errorResponse)
		if err != nil {
			return nil, errors.New("error unmarshalling error!")
		}

		return nil, &WardenError{
			Message:   errorResponse.GetMessage(),
			Data:      errorResponse.GetData(),
			Backtrace: errorResponse.GetBacktrace(),
		}
	}

	responseType := protocol.TypeForMessage(response)
	if message.GetType() != responseType {
		return nil, errors.New(
			fmt.Sprintf(
				"expected message type %s, got %s\n",
				responseType.String(),
				message.GetType().String(),
			),
		)
	}

	err := proto.Unmarshal(message.GetPayload(), response)

	return response, err
}

func (c *connection) readPayload() ([]byte, error) {
	c.readLock.Lock()
	defer c.readLock.Unlock()

	msgHeader, err := c.read.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	msgLen, err := strconv.ParseUint(string(msgHeader[0:len(msgHeader)-2]), 10, 0)
	if err != nil {
		return nil, err
	}

	payload, err := readNBytes(int(msgLen), c.read)
	if err != nil {
		return nil, err
	}

	_, err = readNBytes(2, c.read) // CRLN
	if err != nil {
		return nil, err
	}

	return payload, err
}

func readNBytes(payloadLen int, io *bufio.Reader) ([]byte, error) {
	payload := make([]byte, payloadLen)

	for readCount := 0; readCount < payloadLen; {
		n, err := io.Read(payload[readCount:])
		if err != nil {
			return nil, err
		}

		readCount += n
	}

	return payload, nil
}

func convertEnvironmentVariables(environmentVariables []warden.EnvironmentVariable) []*protocol.EnvironmentVariable {
	convertedEnvironmentVariables := []*protocol.EnvironmentVariable{}

	for _, env := range environmentVariables {
		convertedEnvironmentVariable := &protocol.EnvironmentVariable{
			Key:   proto.String(env.Key),
			Value: proto.String(env.Value),
		}

		convertedEnvironmentVariables = append(
			convertedEnvironmentVariables,
			convertedEnvironmentVariable,
		)
	}

	return convertedEnvironmentVariables
}
