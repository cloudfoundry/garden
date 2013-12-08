// +build linux

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
	"runtime"
	"syscall"

	"github.com/vito/garden/backend/linux_backend/wshd/barrier"
	"github.com/vito/garden/backend/linux_backend/wshd/daemon"
	"github.com/vito/garden/command_runner"
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

// high-level view of the control flow for creating a container:
//
//   P = parent,
//   C = child,
//   D = daemon,
//   * = has control flow
//
//   P C D  action
//   ------------------------------------------------------------------
//   *      create listening socket file
//   *      run hook-parent-before-clone
//   *      clone() process with containerization flags
//     *    wait for signal from parent
//   *      run hook-parent-after-clone
//   *      signal child, wait for signal from child
//     *    run hook-child-before-pivot
//     *    set up root filesystem via pivot_root()
//     *    run hook-child-after-pivot
//     *    save socket file descriptor to shared memory
//     *    exec /sbin/wshd with whitespace for argv[0]
//       *  load socket file descriptor and title from shared memory
//       *  remove all /mnt mount points
//       *  setsid()
//       *  fill in argv[0] with process title
//       *  signal parent
//   *      exit 0
//       *  listen on socket for requests
//
func main() {
	flag.Parse()

	// daemonize if argv[0] is whitespace.
	//
	// This is done instead of a flag mainly because there's no convenient
	// way to set the process title in Go. So instead exec with argv[0] as
	// whitespace and just detect that and fill it in later. If we passed a
	// flag we'd have to clear out argv[1] somehow.
	if os.Args[0][0] == ' ' {
		startDaemon()
		return
	}

	commandRunner := command_runner.New(true)

	fullRunPath := resolvePath(*runPath)
	fullLibPath := resolvePath(*libPath)
	fullRootPath := resolvePath(*rootPath)

	// lock this goroutine to its OS thread so the next Unshare sticks for
	// the full duration; otherwise if we get scheduled to another thread
	// it goes away
	runtime.LockOSThread()

	// unshare mount namespace so the following hooks can modify mount points
	err := syscall.Unshare(CLONE_NEWNS)
	if err != nil {
		log.Fatalln("error unsharing:", err)
	}

	err = commandRunner.Run(&exec.Cmd{Path: path.Join(fullLibPath, "hook-parent-before-clone.sh")})
	if err != nil {
		log.Fatalln("error executing hook-parent-before-clone.sh:", err)
	}

	// create listening socket file descriptor, so we can pass its fd to
	// the container later
	listener, err := net.Listen("unix", path.Join(fullRunPath, "wshd.sock"))
	if err != nil {
		log.Fatalln("error listening:", err)
	}

	socketFile, err := listener.(*net.UnixListener).File()
	if err != nil {
		log.Fatalln("error getting listening file:", err)
	}

	logFile, err := os.Create(path.Join(fullRunPath, "wshd.log"))
	if err != nil {
		log.Fatalln("error creating wshd log file:", err)
	}

	// dup the socket and log file fds; otherwise they'll close-on-exec
	newFD, err := syscall.Dup(int(socketFile.Fd()))
	if err != nil {
		log.Fatalln("error duplicating socket file:", err)
	}

	logFD, err := syscall.Dup(int(logFile.Fd()))
	if err != nil {
		log.Fatalln("error duplicating log file:", err)
	}

	// parent -> child synchronization
	parentBarrier, err := barrier.New()
	if err != nil {
		log.Fatalln("error creating parent barrier:", err)
	}

	// child -> parent synchronization
	childBarrier, err := barrier.New()
	if err != nil {
		log.Fatalln("error creating child barrier:", err)
	}

	// set up the state to temporarily share in-memory with the container
	state := State{
		SocketFD:      newFD,
		LogFD:         logFD,
		ChildBarrier:  childBarrier,
		ParentBarrier: parentBarrier,
	}

	// clone() to create a containerized thread
	pid, err := createContainerizedProcess()
	if err != nil {
		log.Fatalln("error creating child process:", err)
	}

	// we're the clone; execute as the child
	if pid == 0 {
		childRun(state, fullLibPath, fullRootPath, commandRunner)

		panic("unreachable")
	}

	// we're the parent; execute after-clone hook here
	os.Setenv("PID", fmt.Sprintf("%d", pid))

	err = commandRunner.Run(&exec.Cmd{Path: path.Join(fullLibPath, "hook-parent-after-clone.sh")})
	if err != nil {
		log.Fatalln("error executing hook-parent-after-clone.sh:", err)
	}

	// tell the child process to begin, now that the hook is executed
	err = parentBarrier.Signal()
	if err != nil {
		log.Fatalln("error signaling child:", err)
	}

	// wait for the exec'd container (--daemon) to come alive
	err = childBarrier.Wait()
	if err != nil {
		log.Fatalln("error waiting for signal from child:", err)
	}

	// our job is done; the container is daemonized
	os.Exit(0)
}

func resolvePath(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		log.Fatalln("error resolving path:", path, err)
	}

	fullPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		log.Fatalln("error resolving path:", path, err)
	}

	return fullPath
}

func createContainerizedProcess() (int, error) {
	syscall.ForkLock.Lock()
	defer syscall.ForkLock.Unlock()

	var flags uintptr

	flags |= CLONE_NEWNS  // new mount namespace
	flags |= CLONE_NEWUTS // new UTS namespace
	flags |= CLONE_NEWIPC // new inter-process communiucation namespace
	flags |= CLONE_NEWPID // new process pid namespace
	flags |= CLONE_NEWNET // new network namespace

	pid, _, err := syscall.RawSyscall(syscall.SYS_CLONE, flags, 0, 0)
	if err != 0 {
		return 0, err
	}

	return int(pid), nil
}

func childRun(state State, fullLibPath, fullRootPath string, commandRunner command_runner.CommandRunner) {
	argv0 := "/sbin/wshd"

	if *title != "" {
		argv0 = *title
	}

	blankTitle := make([]byte, len(argv0))
	for i := 0; i < len(argv0); i++ {
		blankTitle[i] = ' '
	}

	state.Title = argv0

	// wait until hook-parent-after-clone is done
	err := state.ParentBarrier.Wait()
	if err != nil {
		log.Println("error waiting for signal from parent:", err)
		goto cleanup
	}

	err = commandRunner.Run(&exec.Cmd{Path: path.Join(fullLibPath, "hook-child-before-pivot.sh")})
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

	err = commandRunner.Run(&exec.Cmd{Path: path.Join("/mnt", fullLibPath, "hook-child-after-pivot.sh")})
	if err != nil {
		log.Println("error executing hook-child-after-pivot.sh:", err)
		goto cleanup
	}

	err = SaveStateToSHM(state)
	if err != nil {
		log.Println("error saving state:", err)
		goto cleanup
	}

	// exec wshd on the new filesystem and tell it to daemonize
	err = syscall.Exec("/sbin/wshd", []string{string(blankTitle)}, []string{})
	if err != nil {
		log.Println("error executing /sbin/wshd:", err)
		goto cleanup
	}
cleanup:
	state.ChildBarrier.Fail()
	os.Exit(255)
}

func startDaemon() {
	// load socket file from memory
	state, err := LoadStateFromSHM()
	if err != nil {
		log.Fatalln("error loading state:", err)
	}

	// don't leak file descriptors to child processes
	syscall.CloseOnExec(state.ChildBarrier.FDs[0])
	syscall.CloseOnExec(state.ChildBarrier.FDs[1])
	syscall.CloseOnExec(state.SocketFD)
	syscall.CloseOnExec(state.LogFD)

	logFile := os.NewFile(uintptr(state.LogFD), "wshd.log")

	log.SetOutput(logFile)

	setProcTitle(state.Title)

	// clean up /mnt mount points
	err = umountAll("/mnt")
	if err != nil {
		log.Fatalf("error clearing /mnt mountpoints: %#v\n", err)
	}

	// detach
	_, err = syscall.Setsid()
	if err != nil {
		log.Fatalln("error setting sid:", err)
	}

	socketFile := os.NewFile(uintptr(state.SocketFD), "wshd.sock")

	daemon := daemon.New(socketFile)

	err = state.ChildBarrier.Signal()
	if err != nil {
		log.Fatalln("error signalling parent:", err)
	}

	// start listening on the socket
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
