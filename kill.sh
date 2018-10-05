#!/bin/bash
ps -efo comm,pid,args | grep haproxy.cfg | grep -v grep | awk '{print $2}' | xargs -I {} kill -9 {}
