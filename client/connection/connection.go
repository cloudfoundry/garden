package connection

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/routes"
	"github.com/cloudfoundry-incubator/garden/transport"
	"github.com/tedsuo/rata"
)

var ErrDisconnected = errors.New("disconnected")
var ErrInvalidMessage = errors.New("invalid message payload")

//go:generate counterfeiter . Connection

type Connection interface {
	Ping() error

	Capacity() (garden.Capacity, error)

	Create(spec garden.ContainerSpec) (string, error)
	List(properties garden.Properties) ([]string, error)

	// Destroys the container with the given handle. If the container cannot be
	// found, garden.ContainerNotFoundError is returned. If deletion fails for another
	// reason, another error type is returned.
	Destroy(handle string) error

	Stop(handle string, kill bool) error

	Info(handle string) (garden.ContainerInfo, error)

	StreamIn(handle string, dstPath string, reader io.Reader) error
	StreamOut(handle string, srcPath string) (io.ReadCloser, error)

	LimitBandwidth(handle string, limits garden.BandwidthLimits) (garden.BandwidthLimits, error)
	LimitCPU(handle string, limits garden.CPULimits) (garden.CPULimits, error)
	LimitDisk(handle string, limits garden.DiskLimits) (garden.DiskLimits, error)
	LimitMemory(handle string, limit garden.MemoryLimits) (garden.MemoryLimits, error)

	CurrentBandwidthLimits(handle string) (garden.BandwidthLimits, error)
	CurrentCPULimits(handle string) (garden.CPULimits, error)
	CurrentDiskLimits(handle string) (garden.DiskLimits, error)
	CurrentMemoryLimits(handle string) (garden.MemoryLimits, error)

	Run(handle string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error)
	Attach(handle string, processID uint32, io garden.ProcessIO) (garden.Process, error)

	NetIn(handle string, hostPort, containerPort uint32) (uint32, uint32, error)
	NetOut(handle string, rule garden.NetOutRule) error

	GetProperty(handle string, name string) (string, error)
	SetProperty(handle string, name string, value string) error
	RemoveProperty(handle string, name string) error
}

type connection struct {
	req *rata.RequestGenerator

	dialer func(string, string) (net.Conn, error)

	httpClient        *http.Client
	noKeepaliveClient *http.Client
}

type Error struct {
	StatusCode int
	Message    string
}

func (err Error) Error() string {
	return err.Message
}

func New(network, address string) Connection {
	dialer := func(string, string) (net.Conn, error) {
		return net.DialTimeout(network, address, time.Second)
	}

	return &connection{
		req: rata.NewRequestGenerator("http://api", routes.Routes),

		dialer: dialer,

		httpClient: &http.Client{
			Transport: &http.Transport{
				Dial: dialer,
			},
		},
		noKeepaliveClient: &http.Client{
			Transport: &http.Transport{
				Dial:              dialer,
				DisableKeepAlives: true,
			},
		},
	}
}

func (c *connection) Ping() error {
	return c.do(routes.Ping, nil, &struct{}{}, nil, nil)
}

func (c *connection) Capacity() (garden.Capacity, error) {
	capacity := garden.Capacity{}
	err := c.do(routes.Capacity, nil, &capacity, nil, nil)
	if err != nil {
		return garden.Capacity{}, err
	}

	return capacity, nil
}

func (c *connection) Create(spec garden.ContainerSpec) (string, error) {
	res := struct {
		Handle string `json:"handle"`
	}{}

	err := c.do(routes.Create, spec, &res, nil, nil)
	if err != nil {
		return "", err
	}

	return res.Handle, nil
}

func (c *connection) Stop(handle string, kill bool) error {
	return c.do(
		routes.Stop,
		map[string]bool{
			"kill": kill,
		},
		&struct{}{},
		rata.Params{
			"handle": handle,
		},
		nil,
	)
}

func (c *connection) Destroy(handle string) error {
	return c.do(
		routes.Destroy,
		nil,
		&struct{}{},
		rata.Params{
			"handle": handle,
		},
		nil,
	)
}

func (c *connection) Run(handle string, spec garden.ProcessSpec, processIO garden.ProcessIO) (garden.Process, error) {
	reqBody := new(bytes.Buffer)

	err := transport.WriteMessage(reqBody, spec)
	if err != nil {
		return nil, err
	}

	conn, br, err := c.doHijack(
		routes.Run,
		reqBody,
		rata.Params{
			"handle": handle,
		},
		nil,
		"application/json",
	)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(br)

	firstResponse := &transport.ProcessPayload{}
	err = decoder.Decode(firstResponse)
	if err != nil {
		return nil, err
	}

	p := newProcess(firstResponse.ProcessID, conn)

	go p.streamPayloads(decoder, processIO)

	return p, nil
}

func (c *connection) Attach(handle string, processID uint32, processIO garden.ProcessIO) (garden.Process, error) {
	reqBody := new(bytes.Buffer)

	conn, br, err := c.doHijack(
		routes.Attach,
		reqBody,
		rata.Params{
			"handle": handle,
			"pid":    fmt.Sprintf("%d", processID),
		},
		nil,
		"",
	)

	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(br)

	p := newProcess(processID, conn)

	go p.streamPayloads(decoder, processIO)

	return p, nil
}

func (c *connection) NetIn(handle string, hostPort, containerPort uint32) (uint32, uint32, error) {
	res := &transport.NetInResponse{}

	err := c.do(
		routes.NetIn,
		&transport.NetInRequest{
			Handle:        handle,
			HostPort:      hostPort,
			ContainerPort: containerPort,
		},
		res,
		rata.Params{
			"handle": handle,
		},
		nil,
	)

	if err != nil {
		return 0, 0, err
	}

	return res.HostPort, res.ContainerPort, nil
}

func (c *connection) NetOut(handle string, rule garden.NetOutRule) error {
	return c.do(
		routes.NetOut,
		rule,
		&struct{}{},
		rata.Params{
			"handle": handle,
		},
		nil,
	)
}

func (c *connection) GetProperty(handle string, name string) (string, error) {
	var res struct {
		Value string `json:"value"`
	}

	err := c.do(
		routes.GetProperty,
		nil,
		&res,
		rata.Params{
			"handle": handle,
			"key":    name,
		},
		nil,
	)

	return res.Value, err
}

func (c *connection) SetProperty(handle string, name string, value string) error {
	err := c.do(
		routes.SetProperty,
		map[string]string{
			"value": value,
		},
		&struct{}{},
		rata.Params{
			"handle": handle,
			"key":    name,
		},
		nil,
	)

	if err != nil {
		return err
	}

	return nil
}

func (c *connection) RemoveProperty(handle string, name string) error {
	err := c.do(
		routes.RemoveProperty,
		nil,
		&struct{}{},
		rata.Params{
			"handle": handle,
			"key":    name,
		},
		nil,
	)

	if err != nil {
		return err
	}

	return nil
}

func (c *connection) LimitBandwidth(handle string, limits garden.BandwidthLimits) (garden.BandwidthLimits, error) {
	res := garden.BandwidthLimits{}
	err := c.do(
		routes.LimitBandwidth,
		limits,
		&res,
		rata.Params{
			"handle": handle,
		},
		nil,
	)

	return res, err
}

func (c *connection) CurrentBandwidthLimits(handle string) (garden.BandwidthLimits, error) {
	res := garden.BandwidthLimits{}

	err := c.do(
		routes.CurrentBandwidthLimits,
		nil,
		&res,
		rata.Params{
			"handle": handle,
		},
		nil,
	)

	return res, err
}

func (c *connection) LimitCPU(handle string, limits garden.CPULimits) (garden.CPULimits, error) {
	res := garden.CPULimits{}

	err := c.do(
		routes.LimitCPU,
		limits,
		&res,
		rata.Params{
			"handle": handle,
		},
		nil,
	)

	return res, err
}

func (c *connection) CurrentCPULimits(handle string) (garden.CPULimits, error) {
	res := garden.CPULimits{}

	err := c.do(
		routes.CurrentCPULimits,
		nil,
		&res,
		rata.Params{
			"handle": handle,
		},
		nil,
	)

	return res, err
}

func (c *connection) LimitDisk(handle string, limits garden.DiskLimits) (garden.DiskLimits, error) {
	res := garden.DiskLimits{}

	err := c.do(
		routes.LimitDisk,
		limits,
		&res,
		rata.Params{
			"handle": handle,
		},
		nil,
	)

	return res, err
}

func (c *connection) CurrentDiskLimits(handle string) (garden.DiskLimits, error) {
	res := garden.DiskLimits{}

	err := c.do(
		routes.CurrentDiskLimits,
		nil,
		&res,
		rata.Params{
			"handle": handle,
		},
		nil,
	)

	return res, err
}

func (c *connection) LimitMemory(handle string, limits garden.MemoryLimits) (garden.MemoryLimits, error) {
	res := garden.MemoryLimits{}

	err := c.do(
		routes.LimitMemory,
		limits,
		&res,
		rata.Params{
			"handle": handle,
		},
		nil,
	)

	return res, err
}

func (c *connection) CurrentMemoryLimits(handle string) (garden.MemoryLimits, error) {
	res := garden.MemoryLimits{}

	err := c.do(
		routes.CurrentMemoryLimits,
		nil,
		&res,
		rata.Params{
			"handle": handle,
		},
		nil,
	)

	return res, err
}

func (c *connection) StreamIn(handle string, dstPath string, reader io.Reader) error {
	body, err := c.doStream(
		routes.StreamIn,
		reader,
		rata.Params{
			"handle": handle,
		},
		url.Values{
			"destination": []string{dstPath},
		},
		"application/x-tar",
	)
	if err != nil {
		return err
	}

	return body.Close()
}

func (c *connection) StreamOut(handle string, srcPath string) (io.ReadCloser, error) {
	return c.doStream(
		routes.StreamOut,
		nil,
		rata.Params{
			"handle": handle,
		},
		url.Values{
			"source": []string{srcPath},
		},
		"",
	)
}

func (c *connection) List(filterProperties garden.Properties) ([]string, error) {
	values := url.Values{}
	for name, val := range filterProperties {
		values[name] = []string{val}
	}

	res := &struct {
		Handles []string
	}{}

	if err := c.do(
		routes.List,
		nil,
		&res,
		nil,
		values,
	); err != nil {
		return nil, err
	}

	return res.Handles, nil
}

func (c *connection) Info(handle string) (garden.ContainerInfo, error) {
	res := garden.ContainerInfo{}

	err := c.do(routes.Info, nil, &res, rata.Params{"handle": handle}, nil)
	if err != nil {
		return garden.ContainerInfo{}, err
	}

	return res, nil
}

func (c *connection) do(
	handler string,
	req, res interface{},
	params rata.Params,
	query url.Values,
) error {
	var body io.Reader

	if req != nil {
		buf := new(bytes.Buffer)

		err := transport.WriteMessage(buf, req)
		if err != nil {
			return err
		}

		body = buf
	}

	contentType := ""
	if req != nil {
		contentType = "application/json"
	}

	response, err := c.doStream(
		handler,
		body,
		params,
		query,
		contentType,
	)
	if err != nil {
		return err
	}

	defer response.Close()

	return json.NewDecoder(response).Decode(res)
}

func (c *connection) doStream(
	handler string,
	body io.Reader,
	params rata.Params,
	query url.Values,
	contentType string,
) (io.ReadCloser, error) {
	request, err := c.req.CreateRequest(handler, params, body)
	if err != nil {
		return nil, err
	}

	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	if query != nil {
		request.URL.RawQuery = query.Encode()
	}

	httpResp, err := c.noKeepaliveClient.Do(request)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode > 299 {
		errResponse, err := ioutil.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("bad response: %s", httpResp.Status)
		}

		return nil, Error{httpResp.StatusCode, string(errResponse)}
	}

	return httpResp.Body, nil
}

func (c *connection) doHijack(
	handler string,
	body io.Reader,
	params rata.Params,
	query url.Values,
	contentType string,
) (net.Conn, *bufio.Reader, error) {
	request, err := c.req.CreateRequest(handler, params, body)
	if err != nil {
		return nil, nil, err
	}

	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	if query != nil {
		request.URL.RawQuery = query.Encode()
	}

	conn, err := c.dialer("tcp", "api") // net/addr don't matter here
	if err != nil {
		return nil, nil, err
	}

	client := httputil.NewClientConn(conn, nil)

	httpResp, err := client.Do(request)
	if err != nil {
		return nil, nil, err
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode > 299 {
		httpResp.Body.Close()
		return nil, nil, fmt.Errorf("bad response: %s", httpResp.Status)
	}

	conn, br := client.Hijack()

	return conn, br, nil
}
