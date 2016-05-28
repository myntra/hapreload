A simple tool to add rules to haproxy.cfg and reload the [Haproxy](https://hub.docker.com/_/haproxy/) container. It uses JSON RPC and has Add, Remove and Generate Methods. Please see test.py for usage.

### Usage
```bash
git clone git@github.com:adnaan/hapreload.git
cd hapreload
export DOCKER_HOST="host-ip:port"
go run hapreload.go
```

Example usage

```bash
url = 'http://localhost:34015/haproxy'
addArgs = {'Name':'myapp','Port':'7777','Domain':'.example.com'}
removeArgs = {'Name':'myapp'}
args = {}
print rpc_call(url, "Haproxy.Add", addArgs)
print rpc_call(url, "Haproxy.Remove", removeArgs)
# regenerates the config and reloads the container
print rpc_call(url, "Haproxy.Generate", args)
```

You can put the binary in /path/to/haproxy.cfg. From the docker haproxy page:

```bash
docker run -d --name my-running-haproxy -v /path/to/haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg:ro haproxy:1.5
```
This is a hack. A better solution would be [consul template](https://github.com/hashicorp/consul-template).

This project is inspired from https://github.com/joewilliams/haproxy_join.
