package connection

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"code.google.com/p/goprotobuf/proto"

	"github.com/cloudfoundry-incubator/garden/client/releasenotifier"
	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/transport"
	"github.com/cloudfoundry-incubator/garden/warden"
)

var ErrDisconnected = errors.New("disconnected")
var ErrInvalidMessage = errors.New("invalid message payload")

type Connection interface {
	Close()

	Disconnected() <-chan struct{}

	Capacity() (warden.Capacity, error)

	Create(spec warden.ContainerSpec) (string, error)
	List(properties warden.Properties) ([]string, error)
	Destroy(handle string) error

	Stop(handle string, background, kill bool) error

	Info(handle string) (warden.ContainerInfo, error)

	StreamIn(handle string, dstPath string) (io.WriteCloser, error)
	StreamOut(handle string, srcPath string) (io.Reader, error)

	LimitBandwidth(handle string, limits warden.BandwidthLimits) (warden.BandwidthLimits, error)
	LimitCPU(handle string, limits warden.CPULimits) (warden.CPULimits, error)
	LimitDisk(handle string, limits warden.DiskLimits) (warden.DiskLimits, error)
	LimitMemory(handle string, limit warden.MemoryLimits) (warden.MemoryLimits, error)

	Run(handle string, spec warden.ProcessSpec) (uint32, <-chan warden.ProcessStream, error)
	Attach(handle string, processID uint32) (<-chan warden.ProcessStream, error)

	NetIn(handle string, hostPort, containerPort uint32) (uint32, uint32, error)
	NetOut(handle string, network string, port uint32) error
}

type connection struct {
	messages chan *protocol.Message

	disconnected   chan struct{}
	disconnectOnce *sync.Once

	conn net.Conn

	read *bufio.Reader

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

	messagesR, messagesW := io.Pipe()

	connection := &connection{
		messages: messages,

		disconnected:   make(chan struct{}),
		disconnectOnce: &sync.Once{},

		conn: conn,

		read: bufio.NewReader(messagesR),
	}

	go connection.readMessages(messagesW)

	return connection
}

func (c *connection) Close() {
	c.conn.Close()
}

func (c *connection) Disconnected() <-chan struct{} {
	return c.disconnected
}

func (c *connection) Capacity() (warden.Capacity, error) {
	req := &protocol.CapacityRequest{}
	res := &protocol.CapacityResponse{}

	err := c.roundTrip(req, res)
	if err != nil {
		return warden.Capacity{}, err
	}

	return warden.Capacity{
		MemoryInBytes: res.GetMemoryInBytes(),
		DiskInBytes:   res.GetDiskInBytes(),
		MaxContainers: res.GetMaxContainers(),
	}, nil
}

func (c *connection) Create(spec warden.ContainerSpec) (string, error) {
	req := &protocol.CreateRequest{}

	if spec.Handle != "" {
		req.Handle = proto.String(spec.Handle)
	}

	if spec.RootFSPath != "" {
		req.Rootfs = proto.String(spec.RootFSPath)
	}

	if spec.GraceTime != 0 {
		req.GraceTime = proto.Uint32(uint32(spec.GraceTime.Seconds()))
	}

	if spec.Network != "" {
		req.Network = proto.String(spec.Network)
	}

	for _, bm := range spec.BindMounts {
		var mode protocol.CreateRequest_BindMount_Mode
		var origin protocol.CreateRequest_BindMount_Origin

		switch bm.Mode {
		case warden.BindMountModeRO:
			mode = protocol.CreateRequest_BindMount_RO
		case warden.BindMountModeRW:
			mode = protocol.CreateRequest_BindMount_RW
		}

		switch bm.Origin {
		case warden.BindMountOriginHost:
			origin = protocol.CreateRequest_BindMount_Host
		case warden.BindMountOriginContainer:
			origin = protocol.CreateRequest_BindMount_Container
		}

		req.BindMounts = append(req.BindMounts, &protocol.CreateRequest_BindMount{
			SrcPath: proto.String(bm.SrcPath),
			DstPath: proto.String(bm.DstPath),
			Mode:    &mode,
			Origin:  &origin,
		})
	}

	props := []*protocol.Property{}
	for key, val := range spec.Properties {
		props = append(props, &protocol.Property{
			Key:   proto.String(key),
			Value: proto.String(val),
		})
	}

	req.Properties = props

	res := &protocol.CreateResponse{}

	err := c.roundTrip(req, res)
	if err != nil {
		return "", err
	}

	return res.GetHandle(), nil
}

func (c *connection) Stop(handle string, background, kill bool) error {
	err := c.roundTrip(
		&protocol.StopRequest{
			Handle:     proto.String(handle),
			Background: proto.Bool(background),
			Kill:       proto.Bool(kill),
		},
		&protocol.StopResponse{},
	)

	if err != nil {
		return err
	}

	return nil
}

func (c *connection) Destroy(handle string) error {
	err := c.roundTrip(
		&protocol.DestroyRequest{Handle: proto.String(handle)},
		&protocol.DestroyResponse{},
	)

	if err != nil {
		return err
	}

	return nil
}

func (c *connection) Run(handle string, spec warden.ProcessSpec) (uint32, <-chan warden.ProcessStream, error) {
	err := c.sendMessage(
		&protocol.RunRequest{
			Handle:     proto.String(handle),
			Script:     proto.String(spec.Script),
			Privileged: proto.Bool(spec.Privileged),
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
		return 0, nil, err
	}

	firstResponse := &protocol.ProcessPayload{}

	err = c.readResponse(firstResponse)
	if err != nil {
		return 0, nil, err
	}

	responses := make(chan warden.ProcessStream)

	go c.streamPayloads(responses)

	return firstResponse.GetProcessId(), responses, nil
}

func (c *connection) Attach(handle string, processID uint32) (<-chan warden.ProcessStream, error) {
	err := c.sendMessage(
		&protocol.AttachRequest{
			Handle:    proto.String(handle),
			ProcessId: proto.Uint32(processID),
		},
	)

	if err != nil {
		return nil, err
	}

	responses := make(chan warden.ProcessStream)

	go c.streamPayloads(responses)

	return responses, nil
}

func (c *connection) NetIn(handle string, hostPort, containerPort uint32) (uint32, uint32, error) {
	res := &protocol.NetInResponse{}

	err := c.roundTrip(
		&protocol.NetInRequest{
			Handle:        proto.String(handle),
			HostPort:      proto.Uint32(hostPort),
			ContainerPort: proto.Uint32(containerPort),
		},
		res,
	)

	if err != nil {
		return 0, 0, err
	}

	return res.GetHostPort(), res.GetContainerPort(), nil
}

func (c *connection) NetOut(handle string, network string, port uint32) error {
	err := c.roundTrip(
		&protocol.NetOutRequest{
			Handle:  proto.String(handle),
			Network: proto.String(network),
			Port:    proto.Uint32(port),
		},
		&protocol.NetOutResponse{},
	)

	if err != nil {
		return err
	}

	return nil
}

func (c *connection) LimitBandwidth(handle string, limits warden.BandwidthLimits) (warden.BandwidthLimits, error) {
	res := &protocol.LimitBandwidthResponse{}

	err := c.roundTrip(
		&protocol.LimitBandwidthRequest{
			Handle: proto.String(handle),
			Rate:   proto.Uint64(limits.RateInBytesPerSecond),
			Burst:  proto.Uint64(limits.BurstRateInBytesPerSecond),
		},
		res,
	)

	if err != nil {
		return warden.BandwidthLimits{}, err
	}

	return warden.BandwidthLimits{
		RateInBytesPerSecond:      res.GetRate(),
		BurstRateInBytesPerSecond: res.GetBurst(),
	}, nil
}

func (c *connection) LimitCPU(handle string, limits warden.CPULimits) (warden.CPULimits, error) {
	res := &protocol.LimitCpuResponse{}

	err := c.roundTrip(
		&protocol.LimitCpuRequest{
			Handle:        proto.String(handle),
			LimitInShares: proto.Uint64(limits.LimitInShares),
		},
		res,
	)

	if err != nil {
		return warden.CPULimits{}, err
	}

	return warden.CPULimits{
		LimitInShares: res.GetLimitInShares(),
	}, nil
}

func (c *connection) LimitDisk(handle string, limits warden.DiskLimits) (warden.DiskLimits, error) {
	res := &protocol.LimitDiskResponse{}

	err := c.roundTrip(
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
		res,
	)

	if err != nil {
		return warden.DiskLimits{}, err
	}

	return warden.DiskLimits{
		BlockLimit: res.GetBlockLimit(),
		Block:      res.GetBlock(),
		BlockSoft:  res.GetBlockSoft(),
		BlockHard:  res.GetBlockHard(),

		InodeLimit: res.GetInodeLimit(),
		Inode:      res.GetInode(),
		InodeSoft:  res.GetInodeSoft(),
		InodeHard:  res.GetInodeHard(),

		ByteLimit: res.GetByteLimit(),
		Byte:      res.GetByte(),
		ByteSoft:  res.GetByteSoft(),
		ByteHard:  res.GetByteHard(),
	}, nil
}

func (c *connection) LimitMemory(handle string, limits warden.MemoryLimits) (warden.MemoryLimits, error) {
	res := &protocol.LimitMemoryResponse{}

	err := c.roundTrip(
		&protocol.LimitMemoryRequest{
			Handle:       proto.String(handle),
			LimitInBytes: proto.Uint64(limits.LimitInBytes),
		},
		res,
	)

	if err != nil {
		return warden.MemoryLimits{}, err
	}

	return warden.MemoryLimits{
		LimitInBytes: res.GetLimitInBytes(),
	}, nil
}

func (c *connection) StreamIn(handle string, dstPath string) (io.WriteCloser, error) {
	err := c.roundTrip(
		&protocol.StreamInRequest{
			Handle:  proto.String(handle),
			DstPath: proto.String(dstPath),
		},
		&protocol.StreamInResponse{},
	)

	if err != nil {
		return nil, err
	}

	c.writeLock.Lock()

	return releasenotifier.ReleaseNotifier{
		WriteCloser: transport.NewProtobufStreamWriter(c.conn),
		Callback:    c.writeLock.Unlock,
	}, nil
}

func (c *connection) StreamOut(handle string, srcPath string) (io.Reader, error) {
	err := c.roundTrip(
		&protocol.StreamOutRequest{
			Handle:  proto.String(handle),
			SrcPath: proto.String(srcPath),
		},
		&protocol.StreamOutResponse{},
	)

	if err != nil {
		return nil, err
	}

	c.readLock.Lock()

	return releasenotifier.ReleaseNotifier{
		Reader:   transport.NewProtobufStreamReader(c.read),
		Callback: c.readLock.Unlock,
	}, nil
}

func (c *connection) List(filterProperties warden.Properties) ([]string, error) {
	props := []*protocol.Property{}
	for key, val := range filterProperties {
		props = append(props, &protocol.Property{
			Key:   proto.String(key),
			Value: proto.String(val),
		})
	}

	req := &protocol.ListRequest{Properties: props}
	res := &protocol.ListResponse{}

	err := c.roundTrip(req, res)
	if err != nil {
		return nil, err
	}

	return res.GetHandles(), nil
}

func (c *connection) Info(handle string) (warden.ContainerInfo, error) {
	req := &protocol.InfoRequest{Handle: proto.String(handle)}
	res := &protocol.InfoResponse{}

	err := c.roundTrip(req, res)
	if err != nil {
		return warden.ContainerInfo{}, err
	}

	processIDs := []uint32{}
	for _, pid := range res.GetProcessIds() {
		processIDs = append(processIDs, uint32(pid))
	}

	properties := warden.Properties{}
	for _, prop := range res.GetProperties() {
		properties[prop.GetKey()] = prop.GetValue()
	}

	bandwidthStat := res.GetBandwidthStat()
	cpuStat := res.GetCpuStat()
	diskStat := res.GetDiskStat()
	memoryStat := res.GetMemoryStat()

	return warden.ContainerInfo{
		State:  res.GetState(),
		Events: res.GetEvents(),

		HostIP:      res.GetHostIp(),
		ContainerIP: res.GetContainerIp(),

		ContainerPath: res.GetContainerPath(),

		ProcessIDs: processIDs,

		Properties: properties,

		BandwidthStat: warden.ContainerBandwidthStat{
			InRate:   bandwidthStat.GetInRate(),
			InBurst:  bandwidthStat.GetInBurst(),
			OutRate:  bandwidthStat.GetOutRate(),
			OutBurst: bandwidthStat.GetOutBurst(),
		},

		CPUStat: warden.ContainerCPUStat{
			Usage:  cpuStat.GetUsage(),
			User:   cpuStat.GetUser(),
			System: cpuStat.GetSystem(),
		},

		DiskStat: warden.ContainerDiskStat{
			BytesUsed:  diskStat.GetBytesUsed(),
			InodesUsed: diskStat.GetInodesUsed(),
		},

		MemoryStat: warden.ContainerMemoryStat{
			Cache:                   memoryStat.GetCache(),
			Rss:                     memoryStat.GetRss(),
			MappedFile:              memoryStat.GetMappedFile(),
			Pgpgin:                  memoryStat.GetPgpgin(),
			Pgpgout:                 memoryStat.GetPgpgout(),
			Swap:                    memoryStat.GetSwap(),
			Pgfault:                 memoryStat.GetPgfault(),
			Pgmajfault:              memoryStat.GetPgmajfault(),
			InactiveAnon:            memoryStat.GetInactiveAnon(),
			ActiveAnon:              memoryStat.GetActiveAnon(),
			InactiveFile:            memoryStat.GetInactiveFile(),
			ActiveFile:              memoryStat.GetActiveFile(),
			Unevictable:             memoryStat.GetUnevictable(),
			HierarchicalMemoryLimit: memoryStat.GetHierarchicalMemoryLimit(),
			HierarchicalMemswLimit:  memoryStat.GetHierarchicalMemswLimit(),
			TotalCache:              memoryStat.GetTotalCache(),
			TotalRss:                memoryStat.GetTotalRss(),
			TotalMappedFile:         memoryStat.GetTotalMappedFile(),
			TotalPgpgin:             memoryStat.GetTotalPgpgin(),
			TotalPgpgout:            memoryStat.GetTotalPgpgout(),
			TotalSwap:               memoryStat.GetTotalSwap(),
			TotalPgfault:            memoryStat.GetTotalPgfault(),
			TotalPgmajfault:         memoryStat.GetTotalPgmajfault(),
			TotalInactiveAnon:       memoryStat.GetTotalInactiveAnon(),
			TotalActiveAnon:         memoryStat.GetTotalActiveAnon(),
			TotalInactiveFile:       memoryStat.GetTotalInactiveFile(),
			TotalActiveFile:         memoryStat.GetTotalActiveFile(),
			TotalUnevictable:        memoryStat.GetTotalUnevictable(),
		},
	}, nil
}

func (c *connection) roundTrip(request proto.Message, response proto.Message) error {
	err := c.sendMessage(request)
	if err != nil {
		return err
	}

	return c.readResponse(response)
}

func (c *connection) sendMessage(req proto.Message) error {
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

func (c *connection) readMessages(messagesIn io.WriteCloser) {
	io.Copy(messagesIn, c.conn)
	c.notifyDisconnected()
	messagesIn.Close()
}

func (c *connection) notifyDisconnected() {
	c.disconnectOnce.Do(func() {
		close(c.disconnected)
	})
}

func (c *connection) readResponse(response proto.Message) error {
	c.readLock.Lock()
	defer c.readLock.Unlock()
	return transport.ReadMessage(c.read, response)
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

func (c *connection) streamPayloads(stream chan<- warden.ProcessStream) {
	for {
		payload := &protocol.ProcessPayload{}

		err := c.readResponse(payload)
		if err != nil {
			close(stream)
			break
		}

		if payload.ExitStatus != nil {
			stream <- warden.ProcessStream{
				ExitStatus: payload.ExitStatus,
			}

			close(stream)

			return
		} else {
			var source warden.ProcessStreamSource

			switch payload.GetSource() {
			case protocol.ProcessPayload_stdin:
				source = warden.ProcessStreamSourceStdin
			case protocol.ProcessPayload_stdout:
				source = warden.ProcessStreamSourceStdout
			case protocol.ProcessPayload_stderr:
				source = warden.ProcessStreamSourceStderr
			}

			stream <- warden.ProcessStream{
				Source: source,
				Data:   []byte(payload.GetData()),
			}
		}
	}
}
