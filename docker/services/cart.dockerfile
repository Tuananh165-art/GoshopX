FROM golang:1.24-alpine3.20 AS build
RUN apk --no-cache add gcc g++ make ca-certificates
WORKDIR /go/src/github.com/Tuananh165-art/GoshopX
COPY go.mod go.sum ./
RUN go mod download
COPY cart cart
COPY inventory inventory
COPY pkg pkg
RUN GO111MODULE=on go build -mod mod -o /go/bin/app ./cart/cmd/cart

FROM alpine:3.20
RUN addgroup -S -g 10001 goshopx && adduser -S -D -H -u 10001 -G goshopx goshopx
WORKDIR /usr/bin
COPY --from=build /go/bin .
USER 10001:10001
EXPOSE 8080
CMD ["app"]
