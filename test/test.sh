#!/bin/bash

cd patch
go build --buildmode=plugin -o hp.so patch.go
cd ../main
go build -gcflags="-N -l" -o main main.go
cp ../patch/hp.so .
