#!/bin/bash
rsyslogd
haproxy -p /var/run/haproxy.pid -f /usr/local/etc/haproxy/haproxy.cfg
/usr/bin/hapreload &
tail -f /dev/null