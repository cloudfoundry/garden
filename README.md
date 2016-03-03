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

A rich golang client and server for container creation and management with pluggable backends for [linux](https://github.com/cloudfoundry-incubator/garden-linux/), [windows](https://github.com/cloudfoundry-incubator/garden-windows) and [The Open Container Initiative Spec](https://github.com/cloudfoundry-incubator/guardian/).

[![Coverage Status](https://coveralls.io/repos/cloudfoundry-incubator/garden/badge.png?branch=HEAD)](https://coveralls.io/r/cloudfoundry-incubator/garden?branch=HEAD)

Garden is a platform-agnostic Go API for container creation and management, with pluggable backends for different platforms and runtimes.
This package contains the canonical client, as well as a server package containing an interface to be implemented by backends.

If you're just getting started, you probably want to begin by setting up one of the [backends](#backends) listed below.
If you want to use the Garden client to manage containers, see the [Client API](#client-api) section.

# Backends

Backends implement support for various specific platforms.
So far, the list of backends is as follows:

 - [Garden Linux](https://github.com/cloudfoundry-incubator/garden-linux/) - Linux backend
 - [Guardian](https://github.com/cloudfoundry-incubator/guardian/) - Linux backend using [runc](https://github.com/opencontainers/runc)
 - [Greenhouse](https://github.com/cloudfoundry-incubator/garden-windows) - Windows backend

# Client API

The canonical API for Garden is defined as a collection of Go interfaces.
See the [godoc documentation](http://godoc.org/github.com/cloudfoundry-incubator/garden) for details.

## Example use

_Error checking ignored for brevity._

Import these packages:
```
"github.com/cloudfoundry-incubator/garden"
"github.com/cloudfoundry-incubator/garden/client"
"github.com/cloudfoundry-incubator/garden/client/connection"
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
mkdir -p $GOPATH/src/github.com/cloudfoundry-incubator
cd $GOPATH/src/github.com/cloudfoundry-incubator
git clone git@github.com:cloudfoundry-incubator/garden
cd garden
go get -t -u ./...
go install github.com/onsi/ginkgo/ginkgo
ginkgo -r
```
