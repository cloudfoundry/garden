package main

import (
	"encoding/gob"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"syscall"

	"github.com/vito/garden/backend/linux_backend/wshd/protocol"
)

var socketPath = flag.String(
	"socket",
	"run/wshd.sock",
	"path to gnome socket file",
)

var user = flag.String(
	"user",
	"root",
	"user to run the command as",
)

func main() {
	flag.Parse()

	conn, err := net.Dial("unix", *socketPath)
	if err != nil {
		log.Fatalln(err)
	}

	request := protocol.RequestMessage{
		Argv: flag.Args(),
	}

	encoder := gob.NewEncoder(conn)

	err = encoder.Encode(request)
	if err != nil {
		log.Fatalln("failed writing request:", err)
	}

	log.Println("sent request")

	var b [2048]byte
	var oob [2048]byte

	n, oobn, _, _, err := conn.(*net.UnixConn).ReadMsgUnix(b[:], oob[:])
	if err != nil {
		log.Fatalln("failed to read unix msg:", err, n, oobn)
	}

	log.Println("got response")

	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		log.Fatalln("failed to parse socket control message:", err)
	}

	if len(scms) < 1 {
		log.Fatalln("no socket control messages sent")
	}

	scm := scms[0]

	fds, err := syscall.ParseUnixRights(&scm)
	if err != nil {
		log.Fatalln("failed to parse unix rights", err)
		return
	}

	log.Println("got fds", fds)

	if len(fds) != 4 {
		log.Fatalln("invalid number of fds; need 4, got", len(fds))
	}

	stdin := os.NewFile(uintptr(fds[0]), "stdin")
	stdout := os.NewFile(uintptr(fds[1]), "stdout")
	stderr := os.NewFile(uintptr(fds[2]), "stderr")
	status := os.NewFile(uintptr(fds[3]), "status")

	// err = syscall.SetNonblock(int(os.Stdin.Fd()), false)
	// if err != nil {
	// 	log.Fatalln("failed setting fd nonblock:", err)
	// }

	// err = syscall.SetNonblock(int(os.Stdout.Fd()), false)
	// if err != nil {
	// 	log.Fatalln("failed setting fd nonblock:", err)
	// }

	// err = syscall.SetNonblock(int(os.Stderr.Fd()), false)
	// if err != nil {
	// 	log.Fatalln("failed setting fd nonblock:", err)
	// }

	// for _, fd := range fds {
	// 	err := syscall.SetNonblock(fd, false)
	// 	if err != nil {
	// 		log.Fatalln("failed setting fd nonblock:", err, fd)
	// 	}
	// }

	done := make(chan bool)

	go io.Copy(stdin, os.Stdin)

	go func() {
		io.Copy(os.Stdout, stdout)
		log.Println("stdout done")
		stdout.Close()
		os.Stdout.Close()
		done <- true
	}()

	go func() {
		io.Copy(os.Stderr, stderr)
		log.Println("stderr done")
		stderr.Close()
		os.Stderr.Close()
		done <- true
	}()

	<-done
	<-done

	log.Println("i/o done")

	var exitStatus protocol.ExitStatusMessage

	statusDecoder := gob.NewDecoder(status)

	err = statusDecoder.Decode(&exitStatus)
	if err != nil {
		log.Fatalln("error reading status:", err)
	}

	os.Exit(exitStatus.ExitStatus)
}
