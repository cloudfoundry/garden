```
                                                 ,-.
                                                  ) \
                                              .--'   |
                                             /       /
                                             |_______|
                                            (  O   O  )
                                             {'-(_)-'}
                                           .-{   ^   }-.
                                          /   '.___.'   \
                                         /  |    o    |  \
                                         |__|    o    |__|
                                         (((\_________/)))
                                             \___|___/
                                        jgs.--' | | '--.
                                           \__._| |_.__/
```

**Note**: This repository should be imported as `code.cloudfoundry.org/garden`.

A rich golang client and server for container creation and management with pluggable backends for [linux](https://github.com/cloudfoundry/garden-linux/), [windows](https://github.com/cloudfoundry/garden-windows) and [The Open Container Initiative Spec](https://github.com/cloudfoundry/guardian/).

Garden is a platform-agnostic Go API for container creation and management, with pluggable backends for different platforms and runtimes.
This package contains the canonical client, as well as a server package containing an interface to be implemented by backends.

If you're just getting started, you probably want to begin by setting up one of the [backends](#backends) listed below.
If you want to use the Garden client to manage containers, see the [Client API](#client-api) section.

# Backends

Backends implement support for various specific platforms.
So far, the list of backends is as follows:

 - [Garden Linux](https://github.com/cloudfoundry/garden-linux/) - Linux backend
 - [Guardian](https://github.com/cloudfoundry/guardian/) - Linux backend using [runc](https://github.com/opencontainers/runc)
 - [Greenhouse](https://github.com/cloudfoundry/garden-windows) - Windows backend

# Client API

The canonical API for Garden is defined as a collection of Go interfaces.
See the [godoc documentation](http://godoc.org/code.cloudfoundry.org/garden) for details.

## Example use

_Error checking ignored for brevity._

Import these packages:
```
"code.cloudfoundry.org/garden"
"code.cloudfoundry.org/garden/client"
"code.cloudfoundry.org/garden/client/connection"
```

Create a client:
```
gardenClient := client.New(connection.New("tcp", "127.0.0.1:7777"))
```

Create a container:
```
container, _ := gardenClient.Create(garden.ContainerSpec{})
```

Run a process:
```
buffer := &bytes.Buffer{}
process, _ := container.Run(garden.ProcessSpec{
  User: "alice",
  Path: "echo",
  Args: []string{"hello from the container"},
}, garden.ProcessIO{
  Stdout: buffer,
  Stderr: buffer,
})
exitCode := process.Wait()
fmt.Println(buffer.String())
```

# Development

## Prerequisites

* [go](https://golang.org)
* [git](http://git-scm.com/) (for garden and its dependencies)
* [mercurial](http://mercurial.selenic.com/) (for some other dependencies not using git)

## Running the tests

Assuming go is installed and `$GOPATH` is set:
```
mkdir -p $GOPATH/src/code.cloudfoundry.org
cd $GOPATH/src/code.cloudfoundry.org
git clone git@github.com:cloudfoundry/garden
cd garden
go get -t -u ./...
go install github.com/onsi/ginkgo/ginkgo
ginkgo -r
```
