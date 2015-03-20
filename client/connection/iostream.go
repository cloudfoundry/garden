package connection

import (
	"fmt"
	"io"

	"github.com/cloudfoundry-incubator/garden/routes"
	"github.com/tedsuo/rata"
)

type ioStream struct {
	*connection
	containerHandle string
	processID       uint32
	streamID        uint32
}

// attaches to the stdout and stderr endpoints for a running process
// and copies output to a local io.writers
func (a *ioStream) attach(stdoutW, stderrW io.Writer) error {
	params := rata.Params{
		"handle":   a.containerHandle,
		"pid":      fmt.Sprintf("%d", a.processID),
		"streamid": fmt.Sprintf("%d", a.streamID),
	}

	_, stdout, err := a.doHijack(
		routes.Stdout,
		nil,
		params,
		nil,
		"application/json",
	)

	_, stderr, err := a.doHijack(
		routes.Stderr,
		nil,
		params,
		nil,
		"application/json",
	)

	if stdoutW != nil {
		go io.Copy(stdoutW, stdout)
	}

	if stderrW != nil {
		go io.Copy(stderrW, stderr)
	}

	return err
}
