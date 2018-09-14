FROM alpine:latest
EXPOSE 34015 80 443

RUN echo "http://dl-cdn.alpinelinux.org/alpine/v3.4/main/" > /etc/apk/repositories \
    && apk add --update haproxy vim curl bash tar rsyslog

RUN sed -i -e 's|#$ModLoad imudp|$ModLoad imudp|' /etc/rsyslog.conf
RUN sed -i -e 's|#$UDPServerRun 514|$UDPServerRun 514|' /etc/rsyslog.conf
RUN mkdir -p /etc/rsyslog.d
RUN echo "if (\$programname == 'haproxy') then -/var/log/haproxy.log" > /etc/rsyslog.d/haproxy.conf

# RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build .
ADD hapreload /usr/bin/hapreload
ADD start.sh /usr/bin/start.sh
ADD reload.sh /usr/bin/reload.sh
ADD kill.sh /usr/bin/kill.sh
RUN chmod +x /usr/bin/hapreload /usr/bin/start.sh /usr/bin/reload.sh /usr/bin/kill.sh
CMD ["/usr/bin/start.sh"]
