package connection

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/cloudfoundry-incubator/garden/routes"
	"github.com/tedsuo/rata"
)

type connectionHijacker struct {
	req    *rata.RequestGenerator
	dialer func(string, string) (net.Conn, error)
}

func NewHijackerWithDialer(network, address string) (Hijacker, func(string, string) (net.Conn, error)) {
	dialer := func(string, string) (net.Conn, error) {
		return net.DialTimeout(network, address, time.Second)
	}

	return newHijacker(dialer), dialer
}

func newHijacker(dialer func(string, string) (net.Conn, error)) *connectionHijacker {
	return &connectionHijacker{
		req:    rata.NewRequestGenerator("http://api", routes.Routes),
		dialer: dialer,
	}
}

func (h *connectionHijacker) Hijack(
	handler string,
	body io.Reader,
	params rata.Params,
	query url.Values,
	contentType string,
) (net.Conn, *bufio.Reader, error) {
	request, err := h.req.CreateRequest(handler, params, body)
	if err != nil {
		return nil, nil, err
	}

	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	if query != nil {
		request.URL.RawQuery = query.Encode()
	}

	conn, err := h.dialer("tcp", "api") // net/addr don't matter here
	if err != nil {
		return nil, nil, err
	}

	client := httputil.NewClientConn(conn, nil)

	httpResp, err := client.Do(request)
	if err != nil {
		return nil, nil, err
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode > 299 {
		defer httpResp.Body.Close()

		errRespBytes, err := ioutil.ReadAll(httpResp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("Backend error: Exit status: %d, error reading response body: %s", httpResp.StatusCode, err)
		}
		return nil, nil, fmt.Errorf("Backend error: Exit status: %d, message: %s", httpResp.StatusCode, errRespBytes)
	}

	hijackedConn, hijackedResponseReader := client.Hijack()

	return hijackedConn, hijackedResponseReader, nil
}
