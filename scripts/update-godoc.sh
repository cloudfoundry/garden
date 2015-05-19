#!/usr/bin/env bash
set -e
set -x

repoPath=$(cd $(dirname $BASH_SOURCE)/.. && pwd)

if [ -z $GOROOT ]; then
  export GOROOT=/usr/local/go
  export PATH=$GOROOT/bin:$PATH
fi

if [ -z $GOPATH ]; then
  export GOPATH=$HOME/go
  export PATH=$GOPATH/bin:$PATH
fi

cd $(dirname $0)/..
mkdir -p $GOPATH/src/github.com/cloudfoundry-incubator
ln -s $PWD $GOPATH/src/github.com/cloudfoundry-incubator/garden

cd $GOPATH/src/github.com/cloudfoundry-incubator/garden
go run scripts/update-godoc/main.go
