#!/bin/bash
rsyslogd -i /var/run/rsyslogd.pid
/usr/bin/hapreload > /var/log/hapreload.log 2>&1 &
tail -f /dev/null