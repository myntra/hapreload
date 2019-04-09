#!/bin/bash
PID_LIST=`ps -eo comm,pid,args | grep haproxy.cfg | grep -v grep | awk '{print $2}'`
FINAL_PID=`echo $PID_LIST | sed -E 's/haproxy//g'`
kill -9  $FINAL_PID
exit 0
