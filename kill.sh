#!/bin/bash
ps -efo comm,pid,args | grep haproxy.cfg | grep -v grep | awk '{print $1}' | xargs -I {} kill -9 {}
