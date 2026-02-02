#!/bin/bash
export GOPROXY=https://goproxy.cn,direct
export GOWORK=off
echo "Starting Port Mapping Demo on http://127.0.0.2:1001..."
go run main.go
