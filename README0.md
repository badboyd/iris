# IRIS
Image processing

# Setup and run
- `docker build -t iris-test -f Dockerfile.dev .`
- `docker run -it -p 9090:9090 --name iris -v /Users/vietky/go/src/github.com/701search/imgproxy/configs:/etc/ceph/ -v  -v /Users/vietky/go/src/github.com/701search/iris:/go/src/github.com/701search/iris -e IMGPROXY_USE_CEPH=1 -e IMGPROXY_CEPH_CONF_FILE=/etc/ceph/ceph.conf iris-tes`
- `export AWS_ACCESS_KEY_ID=0EY2C3JCTTPGW5YD6MCS`
- `export AWS_SECRET_ACCESS_KEY=ADoNxpbBqcBydG9bLYOOUhvbI7xwxpab13pEiunw`
- `cd /go/src/github.com/701search/iris && CGO_LDFLAGS_ALLOW="-s|-w|-l" go build -v -o ./iris && ./iris`

# API Interface


# Contributor guide


# TEST