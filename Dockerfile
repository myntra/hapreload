FROM haproxy:1.9.4
EXPOSE 34015 80 443

RUN apt-get update \
    && apt-get install vim curl net-tools rsyslog telnet host socat procps -y
RUN sed -i -e 's|#module(load="imudp")|module(load="imudp")|' /etc/rsyslog.conf
RUN sed -i -e 's|#input(type="imudp" port="514")|input(type="imudp" port="514")|' /etc/rsyslog.conf
RUN mkdir -p /etc/rsyslog.d
RUN echo "if (\$programname == 'haproxy') then -/var/log/haproxy.log" > /etc/rsyslog.d/haproxy.conf

ADD hapreload /usr/bin/hapreload
ADD kill.sh /usr/bin/kill.sh
ADD start.sh /usr/bin/start.sh
ADD reload.sh /usr/bin/reload.sh

ADD haproxy /etc/logrotate.d/haproxy
ADD logrotate /etc/cron.hourly/logrotate

RUN chmod +x /usr/bin/start.sh /usr/bin/reload.sh /usr/bin/kill.sh /etc/logrotate.d/haproxy /etc/cron.hourly/logrotate

CMD ["/usr/bin/start.sh"]