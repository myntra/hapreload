#!/bin/bash
ps -efo comm,pid,args | grep haproxy.cfg | grep -v grep | awk '{print $2}' | sed -e "s/haproxy//" | xargs -I {} kill -9 {}
