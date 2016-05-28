A simple tool to add rules to haproxy.cfg and reload the [Haproxy](https://hub.docker.com/_/haproxy/) container. It uses JSON RPC and has Add, Remove and Generate Methods. Please see test.py for usage.

### Usage
```bash
git clone git@github.com:adnaan/hapreload.git
cd hapreload
export DOCKER_HOST="host-ip:port"
go get
go run hapreload.go
```
The tool uses go package text/template to generate frontend and backend entries.

```bash
const frontendTmpl = `
acl is{{.Acl}} hdr_beg(host) {{.Hostname}}
use_backend {{.Backend}} if is{{.Acl}}
`
const backendTmpl = `
backend {{.Backend}}
  server {{.Backend}} {{.Hostname}}:{{.Port}} check inter 10000
`
```
Since the purpose is to be simple, I would modify the code for a different entry.

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
