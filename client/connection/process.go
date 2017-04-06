package connection

import (
	"sync/atomic"

	"code.cloudfoundry.org/garden"
)

type process struct {
	id string

	processInputStream *processStream

	exitStatus      garden.ProcessStatus
	exitStatusReady chan struct{}
	shouldDoCleanup uint32
}

func newProcess(id string, processInputStream *processStream) *process {
	return &process{
		id:                 id,
		processInputStream: processInputStream,
		exitStatusReady:    make(chan struct{}),
		shouldDoCleanup:    0,
	}
}

func (p *process) ID() string {
	return p.id
}

func (p *process) ExitStatus() chan garden.ProcessStatus {
	ch := make(chan garden.ProcessStatus)
	go func() {
		<-p.exitStatusReady
		ch <- p.exitStatus
	}()

	return ch
}

func (p *process) Wait() (int, error) {
	atomic.StoreUint32(&p.shouldDoCleanup, 1)
	ret := <-p.ExitStatus()
	return ret.Code, ret.Err
}

func (p *process) SetTTY(tty garden.TTYSpec) error {
	return p.processInputStream.SetTTY(tty)
}

func (p *process) Signal(signal garden.Signal) error {
	return p.processInputStream.Signal(signal)
}

func (p *process) exited(exitStatus garden.ProcessStatus) {
	p.exitStatus = exitStatus
	close(p.exitStatusReady)
}

func (p *process) shouldCleanup() bool {
	return atomic.LoadUint32(&p.shouldDoCleanup) > 0
}
