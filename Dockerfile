FROM alpine
RUN apk add --update ca-certificates
EXPOSE 34015
RUN mkdir -p /haproxy
# GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build .
ADD hapreload /hapreload
ADD conf /conf
ENV DOCKER_HOST tcp://localhost:2376
ENV HAPROXY_CONTAINER_NAME haproxy
ENTRYPOINT ["/hapreload"]
CMD ["/hapreload"
