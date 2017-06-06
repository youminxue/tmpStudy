#!/bin/sh
GOPATH=`pwd`
export GOPATH=$GOPATH
export GOROOT=/usr/local/go/
export PATH=$PATH:$GOROOT/bin
GOBIN=/usr/local/go/bin/go
go clean worker
rm bin -rf
rm pkg -rf
