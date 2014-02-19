package garden_runner

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cloudfoundry/gunk/runner_support"
	"github.com/vito/cmdtest"
	"github.com/vito/gordon"
)

type GardenRunner struct {
	Network string
	Addr    string

	DepotPath     string
	BinPath       string
	RootFSPath    string
	SnapshotsPath string

	gardenBin string
	gardenCmd *exec.Cmd

	tmpdir string
}

func New(binPath string, rootFSPath string) (*GardenRunner, error) {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "garden-temp-socker")
	if err != nil {
		return nil, err
	}

	runner := &GardenRunner{
		Network:    "unix",
		Addr:       filepath.Join(tmpdir, "warden.sock"),
		BinPath:    binPath,
		RootFSPath: rootFSPath,
	}

	return runner, runner.Prepare()
}

func (r *GardenRunner) Prepare() error {
	var err error

	r.tmpdir, err = ioutil.TempDir(os.TempDir(), "garden-server")
	if err != nil {
		return err
	}

	compiled, err := cmdtest.Build("github.com/pivotal-cf-experimental/garden")
	if err != nil {
		return err
	}

	r.gardenBin = compiled

	r.DepotPath = filepath.Join(r.tmpdir, "containers")
	r.SnapshotsPath = filepath.Join(r.tmpdir, "snapshots")

	if err := os.Mkdir(r.DepotPath, 0755); err != nil {
		return err
	}

	if err := os.Mkdir(r.SnapshotsPath, 0755); err != nil {
		return err
	}

	return nil
}

func (r *GardenRunner) Start(argv ...string) error {
	gardenArgs := argv
	gardenArgs = append(
		gardenArgs,
		"--listenNetwork", r.Network,
		"--listenAddr", r.Addr,
		"--bin", r.BinPath,
		"--depot", r.DepotPath,
		"--rootfs", r.RootFSPath,
		"--snapshots", r.SnapshotsPath,
		"--debug",
		"--disableQuotas",
	)

	garden := exec.Command(r.gardenBin, gardenArgs...)

	garden.Stdout = os.Stdout
	garden.Stderr = os.Stderr

	_, err := cmdtest.StartWrapped(
		garden,
		runner_support.TeeIfVerbose,
		runner_support.TeeIfVerbose,
	)
	if err != nil {
		return err
	}

	started := make(chan bool, 1)
	stop := make(chan bool, 1)

	go r.waitForStart(started, stop)

	timeout := 10 * time.Second

	r.gardenCmd = garden

	select {
	case <-started:
		return nil
	case <-time.After(timeout):
		stop <- true
		return fmt.Errorf("garden did not come up within %s", timeout)
	}
}

func (r *GardenRunner) Stop() error {
	if r.gardenCmd == nil {
		return nil
	}

	err := r.gardenCmd.Process.Signal(os.Interrupt)
	if err != nil {
		return err
	}

	stopped := make(chan bool, 1)
	stop := make(chan bool, 1)

	go r.waitForStop(stopped, stop)

	timeout := 10 * time.Second

	select {
	case <-stopped:
		r.gardenCmd = nil
		return nil
	case <-time.After(timeout):
		stop <- true
		return fmt.Errorf("garden did not shut down within %s", timeout)
	}
}

func (r *GardenRunner) DestroyContainers() error {
	err := exec.Command(filepath.Join(r.BinPath, "clear.sh"), r.DepotPath).Run()
	if err != nil {
		return err
	}

	if err := os.RemoveAll(r.SnapshotsPath); err != nil {
		return err
	}

	return nil
}

func (r *GardenRunner) TearDown() error {
	err := r.DestroyContainers()
	if err != nil {
		return err
	}

	return os.RemoveAll(r.tmpdir)
}

func (r *GardenRunner) NewClient() gordon.Client {
	return gordon.NewClient(&gordon.ConnectionInfo{
		Network: r.Network,
		Addr:    r.Addr,
	})
}

func (r *GardenRunner) waitForStart(started chan<- bool, stop <-chan bool) {
	for {
		var err error

		conn, dialErr := net.Dial(r.Network, r.Addr)

		if dialErr == nil {
			conn.Close()
		}

		err = dialErr

		if err == nil {
			started <- true
			return
		}

		select {
		case <-stop:
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (r *GardenRunner) waitForStop(stopped chan<- bool, stop <-chan bool) {
	for {
		var err error

		conn, dialErr := net.Dial(r.Network, r.Addr)

		if dialErr == nil {
			conn.Close()
		}

		err = dialErr

		if err != nil {
			stopped <- true
			return
		}

		select {
		case <-stop:
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}
