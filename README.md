A simple tool to add rules to haproxy.cfg and reload the [Haproxy](https://hub.docker.com/_/haproxy/) container. It uses JSON RPC and has Add, Remove and Generate Methods. Please see test.py for usage.

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

### Swarm Example

```bash
# Prerequisite: Startup a swarm following: https://docs.docker.com/engine/userguide/networking/get-started-overlay/
# create overlay network
eval $(docker-machine env --swarm mhs-demo0)
docker network create --driver overlay --subnet=10.0.9.0/24 my-net

# start hapreload container
docker run -d -v /home/docker/haproxy:/haproxy -e "HAPROXY_CONTAINER_NAME=haproxy" \
  -e constraint:node==mhs-demo1 -p 34015:34015 --name hapreload adnaan/hapreload

# Startup a haproxy container in the swarm on mhs-demo1(same node as hapreload)
# start haproxy after hapreload or create   /home/docker/haproxy/haproxy.cfg
# manually on the mhs-demo1 machine.
docker run --net=my-net --name haproxy -p 80:80 -d -v \
  /home/docker/haproxy/haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg \
  -e constraint:node==mhs-demo1 haproxy:1.6

# create another node mhs-demo2 as in the above link
docker-machine ssh mhs-demo2
mkdir -p test && echo "I am myapp" >> test/test && exit
# exit from mhs-demo2 machine

# startup a simple fileserver on the same overlay network
docker run --net=my-net --net-alias=myapp.docker.com --name simplehttpserver -d -v \
  /home/docker/test:/var/www -p 8080 -e constraint:node==mhs-demo2 \
  trinitronx/python-simplehttpserver

# get machine IP : docker-machine ip mhs-demo1
# add to /etc/hosts
# 192.168.99.102 myapp.docker.com (machine IP)

# add rule to haproxy
./test.py http://192.168.99.102/haproxy add
wget -qO- myapp.github.com/test

```

This is a hack. A better solution would be [consul template](https://github.com/hashicorp/consul-template).

This project is inspired from https://github.com/joewilliams/haproxy_join.
