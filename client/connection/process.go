package connection

import (
	"sync/atomic"

	"code.cloudfoundry.org/garden"
)

type process struct {
	id string

	processInputStream *processStream
	status             chan garden.ProcessStatus
	shouldDoCleanup    uint32
}

func newProcess(id string, processInputStream *processStream) *process {
	return &process{
		id:                 id,
		processInputStream: processInputStream,
		status:             make(chan garden.ProcessStatus, 1),
		shouldDoCleanup:    0,
	}
}

func (p *process) ID() string {
	return p.id
}

func (p *process) ExitStatus() chan garden.ProcessStatus {
	return p.status
}

func (p *process) Wait() (int, error) {
	atomic.StoreUint32(&p.shouldDoCleanup, 1)
	ret := <-p.status
	return ret.Code, ret.Err
}

func (p *process) SetTTY(tty garden.TTYSpec) error {
	return p.processInputStream.SetTTY(tty)
}

func (p *process) Signal(signal garden.Signal) error {
	return p.processInputStream.Signal(signal)
}

func (p *process) exited(exitStatus garden.ProcessStatus) {
	//the exited function should only be called once otherwise the
	//line below will block
	p.status <- exitStatus
}

func (p *process) shouldCleanup() bool {
	return atomic.LoadUint32(&p.shouldDoCleanup) > 0
}
