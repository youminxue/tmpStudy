#!/bin/sh
GOPATH=`pwd`
export GOPATH=$PWD:$PWD/../3rdparty:~/workspace/3rdparty
export GOROOT=/usr/local/go
export PATH=$PATH:$GOROOT/bin
GOBIN=/usr/local/go/bin/go
$GOBIN install -ldflags '-s -w' worker
