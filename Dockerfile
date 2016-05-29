FROM looztra/alpine-docker-client
EXPOSE 34015
RUN mkdir /haproxy
ADD conf /default_conf
# GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build .
ADD hapreload /usr/bin/hapreload
RUN chmod +x /usr/bin/hapreload
ENV HAPROXY_CONTAINER_NAME haproxy
ENTRYPOINT ["/usr/bin/hapreload"]
CMD ["/usr/bin/hapreload"]
