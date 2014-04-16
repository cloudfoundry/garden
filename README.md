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

Build the protocol (if you've changed it):
```
$ go get code.google.com/p/gogoprotobuf/{proto,protoc-gen-gogo,gogoproto}
$ make
```

Install ginkgo (used to test garden):
```
$ go install github.com/onsi/ginkgo/ginkgo
```

Run the tests (skipping performance measurements):
```
$ ginkgo -r
```
