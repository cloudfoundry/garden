package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"

	"github.com/vito/garden/backend/linux_backend/wshd/barrier"
	"github.com/vito/garden/backend/linux_backend/wshd/daemon"
)

var runPath = flag.String(
	"run",
	"./run",
	"where to put the gnome daemon .sock file",
)

var rootPath = flag.String(
	"root",
	"./root",
	"root filesystem for the container",
)

var libPath = flag.String(
	"lib",
	"./lib",
	"directory containing hooks",
)

var title = flag.String(
	"title",
	"garden gnome",
	"title for the container gnome daemon",
)

var continueAsChild = flag.Bool(
	"continue",
	false,
	"(internal) continue execution as containerized daemon",
)

func createContainerizedProcess() (int, error) {
	syscall.ForkLock.Lock()
	defer syscall.ForkLock.Unlock()

	pid, _, err := syscall.RawSyscall(
		syscall.SYS_CLONE,
		CLONE_NEWNS|
			CLONE_NEWUTS|
			CLONE_NEWIPC|
			CLONE_NEWPID|
			CLONE_NEWNET,
		0,
		0,
	)

	if err != 0 {
		return 0, err
	}

	return int(pid), nil
}

func childContinue() {
	state, err := LoadStateFromSHM()
	if err != nil {
		log.Fatalln("error loading state:", err)
	}

	childBarrier := &barrier.Barrier{FDs: state.ChildBarrierFDs}

	syscall.CloseOnExec(state.ChildBarrierFDs[0])
	syscall.CloseOnExec(state.ChildBarrierFDs[1])
	syscall.CloseOnExec(state.SocketFD)

	// TODO: set title

	err = umountAll("/mnt")
	if err != nil {
		log.Fatalf("error clearing /mnt mountpoints: %#v\n", err)
	}

	_, err = syscall.Setsid()
	if err != nil {
		log.Fatalln("error setting sid:", err)
	}

	socketFile := os.NewFile(uintptr(state.SocketFD), "wshd.sock")

	daemon := daemon.New(socketFile)

	err = childBarrier.Signal()
	if err != nil {
		log.Fatalln("error signalling parent:", err)
	}

	err = daemon.Start()
	if err != nil {
		log.Fatalln("daemon start error:", err)
	}

	socketFile.Close()

	os.Stdin.Close()
	os.Stdout.Close()
	os.Stderr.Close()

	select {}
}

func main() {
	flag.Parse()

	if *continueAsChild {
		childContinue()
		return
	}

	fullRunPath, err := filepath.Abs(*runPath)
	if err != nil {
		log.Fatalln("error resolving run path:", err)
	}

	fullRunPath, err = filepath.EvalSymlinks(fullRunPath)
	if err != nil {
		log.Fatalln("error resolving run path:", err)
	}

	fullLibPath, err := filepath.Abs(*libPath)
	if err != nil {
		log.Fatalln("error resolving lib path:", err)
	}

	fullLibPath, err = filepath.EvalSymlinks(fullLibPath)
	if err != nil {
		log.Fatalln("error resolving lib path:", err)
	}

	fullRootPath, err := filepath.Abs(*rootPath)
	if err != nil {
		log.Fatalln("error resolving root path:", err)
	}

	fullRootPath, err = filepath.EvalSymlinks(fullRootPath)
	if err != nil {
		log.Fatalln("error resolving root path:", err)
	}

	err = syscall.Unshare(CLONE_NEWNS)
	if err != nil {
		log.Fatalln("error unsharing:", err)
	}

	err = exec.Command(path.Join(*libPath, "hook-parent-before-clone.sh")).Run()
	if err != nil {
		log.Fatalln("error executing hook-parent-before-clone.sh:", err)
	}

	parentBarrier, err := barrier.New()
	if err != nil {
		log.Fatalln("error creating parent barrier:", err)
	}

	childBarrier, err := barrier.New()
	if err != nil {
		log.Fatalln("error creating child barrier:", err)
	}

	listener, err := net.Listen("unix", path.Join(fullRunPath, "wshd.sock"))
	if err != nil {
		log.Fatalln("error listening:", err)
	}

	socketFile, err := listener.(*net.UnixListener).File()
	if err != nil {
		log.Fatalln("error getting listening file:", err)
	}

	newFD, err := syscall.Dup(int(socketFile.Fd()))
	if err != nil {
		log.Fatalln("error duplicating socket file:", err)
	}

	state := State{
		SocketFD:        newFD,
		ChildBarrierFDs: childBarrier.FDs,
	}

	pid, err := createContainerizedProcess()

	if err != nil {
		log.Fatalln("error creating child process:", err)
	}

	if pid == 0 {
		err := parentBarrier.Wait()
		if err != nil {
			log.Println("error waiting for signal from parent:", err)
			goto cleanup
		}

		err = exec.Command(path.Join(fullLibPath, "hook-child-before-pivot.sh")).Run()
		if err != nil {
			log.Println("error executing hook-child-before-pivot.sh:", err)
			goto cleanup
		}

		err = os.Chdir(fullRootPath)
		if err != nil {
			log.Println("error chdir'ing to root path:", err)
			goto cleanup
		}

		err = os.MkdirAll("mnt", 0700)
		if err != nil {
			log.Println("error making mnt:", err)
			goto cleanup
		}

		err = syscall.PivotRoot(".", "mnt")
		if err != nil {
			log.Println("error pivoting root:", err)
			goto cleanup
		}

		err = os.Chdir("/")
		if err != nil {
			log.Println("error chdir'ing to /:", err)
			goto cleanup
		}

		err = exec.Command(path.Join("/mnt", fullLibPath, "hook-child-after-pivot.sh")).Run()
		if err != nil {
			log.Println("error executing hook-child-after-pivot.sh:", err)
			goto cleanup
		}

		err = SaveStateToSHM(state)
		if err != nil {
			log.Println("error saving state:", err)
			goto cleanup
		}

		err = syscall.Exec("/sbin/wshd", []string{"/sbin/wshd", "--continue"}, []string{})
		if err != nil {
			log.Println("error executing /sbin/wshd:", err)
			goto cleanup
		}
	cleanup:
		childBarrier.Fail()
		os.Exit(255)
	}

	os.Setenv("PID", fmt.Sprintf("%d", pid))

	err = exec.Command(path.Join(*libPath, "hook-parent-after-clone.sh")).Run()
	if err != nil {
		log.Fatalln("error executing hook-parent-after-clone.sh:", err)
	}

	err = parentBarrier.Signal()
	if err != nil {
		log.Fatalln("error signaling child:", err)
	}

	err = childBarrier.Wait()
	if err != nil {
		log.Fatalln("error waiting for signal from child:", err)
	}

	os.Exit(0)
}
