#!/bin/bash
rsyslogd
/usr/bin/hapreload -version=v2 > /var/log/hapreload.log 2>&1 &
tail -f /dev/null