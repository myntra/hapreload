#!/usr/bin/env python
import urllib2
import json
import sys


def rpc_call(url, method, args):
    data = json.dumps({
        'id': 1,
        'method': method,
        'params': [args]
    }).encode()
    req = urllib2.Request(url,
        data,
        {'Content-Type': 'application/json'})
    f = urllib2.urlopen(req)
    response = f.read()
    return json.loads(response)

addArgs = {'Services':[{'Name':'myapp','Port':'8080','Domain':'.docker.com'},{'Name':'myapp2','Port':'8181','Domain':'.docker.com'}]}
removeArgs = {'Services':[{'Name':'myapp'},{'Name':'myapp2'}]}
args = {}
if len(sys.argv) != 3:
    print("Usage ./test.py http://HAPROXY-MACHINE-IP:34015/haproxy <add/remove/generate>")
    sys.exit()

url = str(sys.argv[1])
method = str(sys.argv[2])

if method == "add":
    rpc_call(url, "Haproxy.Add", addArgs)
if method == "remove":
    rpc_call(url, "Haproxy.Remove", removeArgs)
if method == "generate":
    rpc_call(url, "Haproxy.Generate", args)
