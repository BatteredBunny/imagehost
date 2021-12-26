#!/bin/env bash
mkdir ./bin
CGO_ENABLED=0 go build -o ./bin/imagehost
tar czf ./bin/build.tar.gz ./bin/imagehost template/ public/