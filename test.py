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

addArgs = {'Name':'myapp','Port':'8080','Domain':'.github.com'}
removeArgs = {'Name':'myapp'}
args = {}
if len(sys.argv) != 2:
    print("Usage ./test.py http://HAPROXY-MACHINE-IP:34015/haproxy add/remove/generate")

url = str(sys.argv[1])
method = str(sys.argv[2])

if method == "add":
    print rpc_call(url, "Haproxy.Add", addArgs)
if method == "remove":
    print rpc_call(url, "Haproxy.Remove", removeArgs)
if method == "generate":
    print rpc_call(url, "Haproxy.Generate", args)
