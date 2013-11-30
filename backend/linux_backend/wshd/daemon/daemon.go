package daemon

import (
	"bytes"
	"encoding/gob"
	"log"
	"net"
	"os"
	"os/exec"
	"syscall"

	"github.com/vito/garden/backend/linux_backend/wshd/protocol"
)

type Daemon struct {
	socket *os.File
}

func New(socket *os.File) *Daemon {
	return &Daemon{socket}
}

func (d *Daemon) Start() error {
	listener, err := net.FileListener(d.socket)
	if err != nil {
		return err
	}

	go d.handleConnections(listener)

	return nil
}

func (d *Daemon) handleConnections(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("daemon error accepting connection:", err)
			continue
		}

		go d.serveConnection(conn.(*net.UnixConn))
	}
}

func (d *Daemon) serveConnection(conn *net.UnixConn) {
	defer conn.Close()

	decoder := gob.NewDecoder(conn)

	var requestMessage protocol.RequestMessage

	err := decoder.Decode(&requestMessage)
	if err != nil {
		log.Println("failed reading request:", err)
		return
	}

	stdinOut, stdinIn, err := os.Pipe()
	if err != nil {
		log.Println("failed making stdin pipe", err)
		return
	}

	stdoutOut, stdoutIn, err := os.Pipe()
	if err != nil {
		log.Println("failed making stdout pipe", err)
		return
	}

	stderrOut, stderrIn, err := os.Pipe()
	if err != nil {
		log.Println("failed making stderr pipe", err)
		return
	}

	statusOut, statusIn, err := os.Pipe()
	if err != nil {
		log.Println("failed making status pipe", err)
		return
	}

	defer stdinOut.Close()
	defer stdinIn.Close()
	defer stdoutOut.Close()
	defer stdoutIn.Close()
	defer stderrOut.Close()
	defer stderrIn.Close()

	response := new(bytes.Buffer)

	encoder := gob.NewEncoder(response)

	err = encoder.Encode(protocol.ResponseMessage{})
	if err != nil {
		log.Println("failed writing response:", err)
		return
	}

	rights := syscall.UnixRights(
		int(stdinIn.Fd()),
		int(stdoutOut.Fd()),
		int(stderrOut.Fd()),
		int(statusOut.Fd()),
	)

	_, _, err = conn.WriteMsgUnix(response.Bytes(), rights, nil)
	if err != nil {
		log.Println("failed sending unix message:", err)
		return
	}

	cmd := &exec.Cmd{
		Path: requestMessage.Argv[0],
		Args: requestMessage.Argv,

		Env: []string{
			"PATH=/sbin:/bin:/usr/sbin:/usr/bin",
		},

		Stdin:  stdinOut,
		Stdout: stdoutIn,
		Stderr: stderrIn,

		SysProcAttr: &syscall.SysProcAttr{
			Setsid: true,
		},
	}

	err = cmd.Start()
	if err != nil {
		defer statusIn.Close()
		defer statusOut.Close()

		log.Println("failed starting command:", err)

		writeStatus(statusIn, 255)

		return
	}

	go func() {
		defer statusIn.Close()
		defer statusOut.Close()

		cmd.Wait()

		exitStatus := 255

		if cmd.ProcessState != nil {
			exitStatus = int(cmd.ProcessState.Sys().(syscall.WaitStatus) % 255)
		}

		writeStatus(statusIn, exitStatus)
	}()
}

func writeStatus(statusIn *os.File, exitStatus int) {
	encoder := gob.NewEncoder(statusIn)

	err := encoder.Encode(protocol.ExitStatusMessage{exitStatus})
	if err != nil {
		log.Println("failed writing exit status:", err)
	}
}
