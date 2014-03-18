package fake_gordon

import (
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/nu7hatch/gouuid"
	"github.com/vito/gordon"
	"github.com/vito/gordon/warden"
	"io/ioutil"
	"os"
	"sync"
)

type FakeGordon struct {
	Connected    bool
	ConnectError error

	createdHandles []string
	CreateError    error

	stoppedHandles []string
	StopError      error

	destroyedHandles []string
	DestroyError     error

	SpawnError error

	LinkError error

	NetInError error

	memoryLimits     []Limit
	limitMemoryError error

	GetMemoryLimitError error

	diskLimits     []DiskLimit
	limitDiskError error

	GetDiskLimitError error

	ListError error

	infoError    error
	infoResponse *warden.InfoResponse

	AttachError error

	scriptsThatRan              []*RunningScript
	runCallbacks                map[*RunningScript]RunCallback
	runReturnProcessID          uint32
	runReturnProcessPayloadChan <-chan *warden.ProcessPayload
	runReturnError              error

	copiedIn    []*CopiedIn
	copyInError error

	copiedOut                     []*CopiedOut
	fileContentToProvideOnCopyOut []byte
	copyOutError                  error

	lock *sync.Mutex
}

type RunCallback func() (uint32, <-chan *warden.ProcessPayload, error)

type RunningScript struct {
	Handle         string
	Script         string
	ResourceLimits gordon.ResourceLimits
}

type CopiedIn struct {
	Handle string
	Src    string
	Dst    string
}

type CopiedOut struct {
	Handle string
	Src    string
	Dst    string
	Owner  string
}

type Limit struct {
	Handle string
	Limit  uint64
}

type DiskLimit struct {
	Handle string
	Limits gordon.DiskLimits
}

func New() *FakeGordon {
	f := &FakeGordon{}
	f.Reset()
	return f
}

func (f *FakeGordon) Reset() {
	f.lock = &sync.Mutex{}
	f.Connected = false
	f.ConnectError = nil

	f.createdHandles = []string{}
	f.CreateError = nil

	f.stoppedHandles = []string{}
	f.StopError = nil

	f.destroyedHandles = []string{}
	f.DestroyError = nil

	f.SpawnError = nil
	f.LinkError = nil
	f.NetInError = nil
	f.GetMemoryLimitError = nil
	f.GetDiskLimitError = nil
	f.ListError = nil
	f.AttachError = nil

	f.infoError = nil

	f.limitMemoryError = nil
	f.limitDiskError = nil
	f.memoryLimits = []Limit{}
	f.diskLimits = []DiskLimit{}

	f.scriptsThatRan = make([]*RunningScript, 0)
	f.runCallbacks = make(map[*RunningScript]RunCallback)
	f.runReturnProcessID = 0
	f.runReturnError = nil

	f.copyInError = nil
	f.copyOutError = nil
	f.copiedIn = []*CopiedIn{}
	f.copiedOut = []*CopiedOut{}
	f.fileContentToProvideOnCopyOut = []byte{}
}

func (f *FakeGordon) Connect() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.Connected = true
	return f.ConnectError
}

func (f *FakeGordon) Create() (*warden.CreateResponse, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if f.CreateError != nil {
		return nil, f.CreateError
	}

	handleUuid, _ := uuid.NewV4()
	handle := handleUuid.String()[:11]

	f.createdHandles = append(f.createdHandles, handle)

	return &warden.CreateResponse{
		Handle: proto.String(handle),
	}, nil
}

func (f *FakeGordon) CreatedHandles() []string {
	f.lock.Lock()
	defer f.lock.Unlock()

	return f.createdHandles
}

func (f *FakeGordon) Stop(handle string, background, kill bool) (*warden.StopResponse, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if f.StopError != nil {
		return nil, f.StopError
	}

	f.stoppedHandles = append(f.stoppedHandles, handle)

	return &warden.StopResponse{}, nil
}

func (f *FakeGordon) StoppedHandles() []string {
	f.lock.Lock()
	defer f.lock.Unlock()

	return f.stoppedHandles
}

func (f *FakeGordon) Destroy(handle string) (*warden.DestroyResponse, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if f.DestroyError != nil {
		return nil, f.DestroyError
	}

	f.destroyedHandles = append(f.destroyedHandles, handle)

	return &warden.DestroyResponse{}, nil
}

func (f *FakeGordon) DestroyedHandles() []string {
	f.lock.Lock()
	defer f.lock.Unlock()

	return f.destroyedHandles
}

func (f *FakeGordon) NetIn(handle string) (*warden.NetInResponse, error) {
	panic("NOOP!")
	return nil, f.NetInError
}

func (f *FakeGordon) MemoryLimits() []Limit {
	f.lock.Lock()
	defer f.lock.Unlock()

	return f.memoryLimits
}

func (f *FakeGordon) SetLimitMemoryError(err error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.limitMemoryError = err
}

func (f *FakeGordon) LimitMemory(handle string, limit uint64) (*warden.LimitMemoryResponse, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.memoryLimits = append(f.memoryLimits, Limit{
		Handle: handle,
		Limit:  limit,
	})

	return nil, f.limitMemoryError
}

func (f *FakeGordon) GetMemoryLimit(handle string) (uint64, error) {
	panic("NOOP!")
	return 0, f.GetMemoryLimitError
}

func (f *FakeGordon) DiskLimits() []DiskLimit {
	f.lock.Lock()
	defer f.lock.Unlock()

	return f.diskLimits
}

func (f *FakeGordon) SetLimitDiskError(err error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.limitDiskError = err
}

func (f *FakeGordon) LimitDisk(handle string, limits gordon.DiskLimits) (*warden.LimitDiskResponse, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.diskLimits = append(f.diskLimits, DiskLimit{
		Handle: handle,
		Limits: limits,
	})

	return nil, f.limitDiskError
}

func (f *FakeGordon) GetDiskLimit(handle string) (uint64, error) {
	panic("NOOP!")
	return 0, f.GetDiskLimitError
}

func (f *FakeGordon) List() (*warden.ListResponse, error) {
	panic("NOOP!")
	return nil, f.ListError
}

func (f *FakeGordon) Info(handle string) (*warden.InfoResponse, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	return f.infoResponse, f.infoError
}

func (f *FakeGordon) SetInfoError(err error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.infoError = err
}

func (f *FakeGordon) SetInfoResponse(response *warden.InfoResponse) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.infoResponse = response
}

func (f *FakeGordon) CopyIn(handle, src, dst string) (*warden.CopyInResponse, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.copyInError != nil {
		return nil, f.copyInError
	}

	f.copiedIn = append(f.copiedIn, &CopiedIn{
		Handle: handle,
		Src:    src,
		Dst:    dst,
	})

	return &warden.CopyInResponse{}, nil
}

func (f *FakeGordon) ThingsCopiedIn() []*CopiedIn {
	f.lock.Lock()
	defer f.lock.Unlock()

	return f.copiedIn
}

func (f *FakeGordon) SetCopyInErr(err error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.copyInError = err
}

func (f *FakeGordon) CopyOut(handle, src, dst, owner string) (*warden.CopyOutResponse, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.copyOutError != nil {
		return nil, f.copyOutError
	}

	f.copiedOut = append(f.copiedOut, &CopiedOut{
		Handle: handle,
		Src:    src,
		Dst:    dst,
		Owner:  owner,
	})

	if len(f.fileContentToProvideOnCopyOut) > 0 {
		ioutil.WriteFile(dst, f.fileContentToProvideOnCopyOut, os.ModePerm)
	}

	return &warden.CopyOutResponse{}, nil
}

func (f *FakeGordon) ThingsCopiedOut() []*CopiedOut {
	f.lock.Lock()
	defer f.lock.Unlock()

	return f.copiedOut
}

func (f *FakeGordon) SetCopyOutErr(err error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.copyOutError = err
}

func (f *FakeGordon) SetCopyOutFileContent(data []byte) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.fileContentToProvideOnCopyOut = data
}

func (f *FakeGordon) Attach(handle string, jobID uint32) (<-chan *warden.ProcessPayload, error) {
	panic("NOOP!")
	return nil, f.AttachError
}

func (f *FakeGordon) ScriptsThatRan() []*RunningScript {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.scriptsThatRan
}

func (f *FakeGordon) SetRunReturnValues(processID uint32, processPayloadChan <-chan *warden.ProcessPayload, err error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.runReturnProcessID = processID
	f.runReturnProcessPayloadChan = processPayloadChan
	f.runReturnError = err
}

func (f *FakeGordon) WhenRunning(handle string, script string, resourceLimits gordon.ResourceLimits, callback RunCallback) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.runCallbacks[&RunningScript{handle, script, resourceLimits}] = callback
}

func (f *FakeGordon) Run(handle string, script string, resourceLimits gordon.ResourceLimits) (uint32, <-chan *warden.ProcessPayload, error) {
	f.lock.Lock()

	f.scriptsThatRan = append(f.scriptsThatRan, &RunningScript{
		Handle:         handle,
		Script:         script,
		ResourceLimits: resourceLimits,
	})

	f.lock.Unlock()

	for ro, cb := range f.runCallbacks {
		if (ro.Handle == "" || ro.Handle == handle) && ro.Script == script {
			return cb()
		}
	}

	return f.runReturnProcessID, f.runReturnProcessPayloadChan, f.runReturnError
}
