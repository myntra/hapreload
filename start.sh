#!/bin/bash
rsyslogd
/usr/bin/hapreload > /var/log/hapreload.log 2>&1 &
tail -f /dev/null