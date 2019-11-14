#!/bin/bash
rsyslogd
touch /var/log/haproxy.log /var/log/hapreload.log
/usr/bin/hapreload -version=v2 > /var/log/hapreload.log 2>&1 &
tail -f /var/log/haproxy.log /var/log/hapreload.log
