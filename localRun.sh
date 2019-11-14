#!/bin/bash
set -e
rm -rf $HOME/haproxy
mkdir -p $HOME/haproxy
cp -r conf $HOME/haproxy
docker rm -f hap|true
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build .
docker build -t hapreload .
docker run -d -v $HOME/haproxy:/usr/local/etc/haproxy -p 8888:80 -p 443:443 -p 34015:34015 -p 3480:3480 --name hap hapreload
sleep 3
docker logs -f hap
