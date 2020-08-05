#!/bin/bash
rsyslogd -i /var/run/rsyslogd.pid
service cron start
/usr/bin/hapreload > /var/log/hapreload.log 2>&1 &
tail -f /dev/null