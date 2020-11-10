#!/usr/bin/env bash

current_path=$(pwd)

# TODO: Should use go mod version.
cd $GOPATH/src/github.com/influxdata/influxdb1-client/v2

# https://github.com/matryer/moq
moq -out=$current_path/influxdb_client_mock_test.go -pkg=influxdb . Client
