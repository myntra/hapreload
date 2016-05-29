A simple tool to add rules to haproxy.cfg and reload the [Haproxy](https://hub.docker.com/_/haproxy/) container. It uses JSON RPC and has Add, Remove, Generate Methods. Please see test.py for usage.

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

### Methods

```bash
url = 'http://localhost:34015/haproxy'
addArgs = {'Name':'myapp','Port':'7777','Domain':'.docker.com'}
removeArgs = {'Name':'myapp'}
args = {}
print rpc_call(url, "Haproxy.Add", addArgs)
print rpc_call(url, "Haproxy.Remove", removeArgs)
# regenerates the config and reloads the container
print rpc_call(url, "Haproxy.Generate", args)
```

Methods to modify global,default and frontend do no exist. Assuming it's one off, we do this manually right now.

### Swarm Example

```bash
# Prerequisite: Setup a swarm: https://docs.docker.com/engine/userguide/networking/get-started-overlay/
# create overlay network
eval $(docker-machine env --swarm mhs-demo0)
docker network create --driver overlay --subnet=10.0.9.0/24 my-net

# Start hapreload container
docker run -d -v /home/docker/haproxy:/haproxy \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e "HAPROXY_CONTAINER_NAME=haproxy" \
  -e constraint:node==mhs-demo1 -p 34015:34015 --name hapreload adnaan/hapreload

# Startup a haproxy container in the swarm on mhs-demo1(same node as hapreload)
# start haproxy after hapreload or create   /home/docker/haproxy/haproxy.cfg
# manually on the mhs-demo1 machine.
docker run --net=my-net --name haproxy -p 80:80 -d -v \
  /home/docker/haproxy/:/usr/local/etc/haproxy \
  -e constraint:node==mhs-demo1 haproxy:1.6

# Startup a simple fileserver on the same overlay network
# create another node mhs-demo2 as in the above link
docker-machine ssh mhs-demo2
mkdir -p test && echo "I am myapp" >> test/test && exit
# exit from mhs-demo2 machine

docker run --net=my-net --net-alias=myapp.docker.com --name simplehttpserver -d -v \
  /home/docker/test:/var/www -p 8080 -e constraint:node==mhs-demo2 \
  trinitronx/python-simplehttpserver

# get machine IP : docker-machine ip mhs-demo1
# add to /etc/hosts
# 192.168.99.102 myapp.docker.com (machine IP)

# add rule to haproxy
./test.py http://192.168.99.102:34015/haproxy add && curl myapp.docker.com/test
  I am myapp

# remove rule from haproxy
./test.py http://192.168.99.102:34015/haproxy remove && curl myapp.docker.com/test
  <html><body><h1>503 Service Unavailable</h1>
  No server is available to handle this request.
  </body></html>

```

### Build

[Go](http://golang.org/doc/install.html) needs to be installed. If development machine is OSX, please do a cross platform build, as hapreload runs on an alpine image.

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build .
```
A linux binary has been checked into this repo.

### Roadmap

Apart from bug fixes, none. This is a hack. A better solution would be [consul template](https://github.com/hashicorp/consul-template).

This project is inspired from https://github.com/joewilliams/haproxy_join.
