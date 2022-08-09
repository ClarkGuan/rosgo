#!/bin/bash
source /opt/ros/kinetic/setup.bash
export PATH=$PWD/bin:/usr/local/go/bin:$PATH
export GOPATH=$PWD:/usr/local/go

roscore &
go install github.com/ClarkGuan/rosgo/gengo
go generate github.com/ClarkGuan/rosgo/test/test_message
go test github.com/ClarkGuan/rosgo/xmlrpc
go test github.com/ClarkGuan/rosgo/ros
go test github.com/ClarkGuan/rosgo/test/test_message

