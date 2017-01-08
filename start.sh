#!/bin/bash
rsyslogd
/usr/bin/hapreload > /var/log/hapreload.log 2>&1 &
haproxy -p /var/run/haproxy.pid -f /usr/local/etc/haproxy/haproxy.cfg
tail -f /dev/null