#!/bin/bash
oldPid=`cat /var/run/haproxy.pid`
#If the Process is running then only it will be killed
kill -0 $oldPid
if [ $? == 0 ];
then
    kill -9 $oldPid
        if [ $? != 0 ];
        then
            exit 1
        fi
fi