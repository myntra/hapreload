A simple tool to add rules to haproxy.cfg and reload the [Haproxy](https://hub.docker.com/_/haproxy/) container. It uses JSON RPC and has Add, Remove and Generate Methods. Please see test.py for usage.

### Usage
```bash
git clone git@github.com:adnaan/hapreload.git
cd hapreload
export DOCKER_HOST="host-ip:port"
export HAPROXY_CONTAINER_NAME="haproxy"
go get
go run hapreload.go
# GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build .
# ./hapreload
```
The tool uses go package text/template to generate frontend and backend entries.

```bash
const frontendTmpl = "
acl is{{.Acl}} hdr_beg(host) {{.Hostname}}
use_backend {{.Backend}} if is{{.Acl}}
"
const backendTmpl = "
backend {{.Backend}}
  server {{.Backend}} {{.Hostname}}:{{.Port}} check inter 10000
"
```
Since the purpose is to be simple, I would modify the code for a different entry.

Methods

```bash
url = 'http://localhost:34015/haproxy'
addArgs = {'Name':'myapp','Port':'7777','Domain':'.github.com'}
removeArgs = {'Name':'myapp'}
args = {}
print rpc_call(url, "Haproxy.Add", addArgs)
print rpc_call(url, "Haproxy.Remove", removeArgs)
# regenerates the config and reloads the container
print rpc_call(url, "Haproxy.Generate", args)
```

Example Usage

```bash
# Prerequisite: Startup a swarm following: https://docs.docker.com/engine/userguide/networking/get-started-overlay/
git clone https://github.com/adnaan/hapreload
cd hapreload
touch haproxy.cfg
mkdir -p test
echo "I am myapp" >> test/test
./hapreload

# In another terminal, as mentioned in the above link
# create overlay network
eval $(docker-machine env --swarm mhs-demo0)
docker network create --driver overlay --subnet=10.0.9.0/24 my-net
# startup a haproxy container in the swarm
docker run --net=my-net --name haproxy -p 80:80 -d -v  \
  -e constraint:node==mhs-demo1 \
  /home/docker/hapreload/haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg haproxy:1.6

# startup a simple fileserver on the same overlay network
docker run --net=my-net --net-alias=myapp.github.com --name simplehttpserver -d -v \
  /home/docker/hapreload/test:/var/www -p 8080 -e constraint:node==mhs-demo2 \
  trinitronx/python-simplehttpserver

# get machine IP : docker-machine ip mhs-demo1
# add to /etc/hosts
# myapp.github.com 192.168.99.102 (machine IP)

# add rule to haproxy
./test.py http://192.168.99.102/haproxy add
wget -qO- myapp.github.com/test

```

This is a hack. A better solution would be [consul template](https://github.com/hashicorp/consul-template).

This project is inspired from https://github.com/joewilliams/haproxy_join.
