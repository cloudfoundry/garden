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

Warden in Go, because why not.

* [![Build Status](https://travis-ci.org/cloudfoundry-incubator/garden.png?branch=master)](https://travis-ci.org/cloudfoundry-incubator/garden)
* [![Coverage Status](https://coveralls.io/repos/cloudfoundry-incubator/garden/badge.png?branch=HEAD)](https://coveralls.io/r/cloudfoundry-incubator/garden?branch=HEAD)
* [Tracker](https://www.pivotaltracker.com/s/projects/962374)
* [Warden](https://github.com/cloudfoundry/warden)

# Backends

Garden provides a platform-neutral API for containerization. Backends implement support for various specific platforms. So far, the list of backends is as follows:

 - [Garden Linux](https://github.com/cloudfoundry-incubator/garden-linux/) - Linux Backend

# Garden API

The canonical API for Garden is defined as a collection of Go interfaces. See the [godoc documentation](http://godoc.org/github.com/cloudfoundry-incubator/garden) for details.

For convenience during Garden development, Garden also supports a REST API which may be used to "kick the tyres". The REST API is not supported.

For example, if [Garden Linux](https://github.com/cloudfoundry-incubator/garden-linux) is deployed to `localhost` and configured to listen on port `7777`, the following commands may be used to kick its tyres:
```sh
# list containers (should be empty)
curl http://127.0.0.1:7777/containers

# create a container
curl -H "Content-Type: application/json" \
  -XPOST http://127.0.0.1:7777/containers \
  -d '{"rootfs":"docker:///busybox"}'

# list containers (should list the handle returned above)
curl http://127.0.0.1:7777/containers

# spawn a process
#
# curl will choke here as the protocol is hijacked, but...it probably worked.
curl -H "Content-Type: application/json" \
  -XPOST http://127.0.0.1:7777/containers/${handle}/processes \
  -d '{"path":"sleep","args":["10"]}'
```

See [REST API examples](doc/garden-api.md) for more.

# Testing

## Pre-requisites

* [git](http://git-scm.com/) (for garden and its dependencies on github)
* [mercurial](http://mercurial.selenic.com/) (for some dependencies not on github)

Make a directory to contain go code:
```
$ mkdir ~/go
```

From now on, we assume this directory is in `/root/go`.

Install Go 1.2.1 or later. For example, install [gvm](https://github.com/moovweb/gvm) and issue:
```
$ gvm install go1.2.1
$ gvm use go1.2.1
```

Extend `$GOPATH` and `$PATH`:
```
$ export GOPATH=/root/go:$GOPATH
$ export PATH=$PATH:/root/go/bin
```

Install [godep](https://github.com/kr/godep) (used to manage garden's dependencies):
```
$ go get github.com/kr/godep
```

Get garden and its dependencies:
```
$ go get github.com/cloudfoundry-incubator/garden
```

and in this directory, re-build the protocol (if you've changed it) see [below](#building-the-protocol).

Install ginkgo (used to test garden):
```
$ go install github.com/onsi/ginkgo/ginkgo
```

Run the tests (skipping performance measurements):
```
$ ginkgo -r
```

# Building the protocol

## Pre-requisite

* [protoc](https://developers.google.com/protocol-buffers/docs/downloads);
    download the archive, unpack it into a directory with no spaces in the full path; and build according to INSTALL.txt.

To build the protocol:
```
$ go get code.google.com/p/gogoprotobuf/{proto,protoc-gen-gogo,gogoproto}
$ make protocol
```