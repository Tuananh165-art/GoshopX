FROM golang:1.26.6-alpine3.23 AS build
RUN apk --no-cache add gcc g++ make ca-certificates git
WORKDIR /go/src/github.com/Tuananh165art/GoshopX
COPY go.mod go.sum ./
# GitHub-hosted runners occasionally receive 403 responses from proxy.golang.org.
# Keep Go checksum verification enabled, then fall back to fetching public modules
# directly from their VCS origins only when the proxy download fails.
RUN go mod download || GOPROXY=direct go mod download
COPY admin admin
COPY pkg pkg
RUN GO111MODULE=on go build -mod=mod -o /go/bin/app ./admin/cmd/admin

FROM alpine:3.23
RUN addgroup -S -g 10001 goshopx && adduser -S -D -H -u 10001 -G goshopx goshopx
WORKDIR /usr/bin
COPY --from=build /go/bin/app ./app
USER 10001:10001
EXPOSE 8080
CMD ["./app"]
