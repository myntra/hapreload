#!/bin/bash
haproxy -p /var/run/haproxy.pid -f /usr/local/etc/haproxy/haproxy.cfg -sf $(cat /var/run/haproxy.pid)