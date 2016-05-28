#!/usr/bin/env python
import urllib2
import json


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

url = 'http://localhost:34015/haproxy'
addArgs = {'Name':'myapp','Port':'7777','Domain':'.example.com'}
removeArgs = {'Name':'myapp'}
args = {}
print rpc_call(url, "Haproxy.Add", addArgs)
#print rpc_call(url, "Haproxy.Remove", removeArgs)
#print rpc_call(url, "Haproxy.Generate", args)
