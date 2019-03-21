docker rm -f hap
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build .
docker build -t hapreload .
docker run -d -v $HOME/haproxy:/usr/local/etc/haproxy -p 8888:80 -p 443:443 -p 34015:34015 --name hap hapreload