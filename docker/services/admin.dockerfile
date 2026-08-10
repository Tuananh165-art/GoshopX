FROM golang:1.24-alpine3.20 AS build
RUN apk --no-cache add gcc g++ make ca-certificates
WORKDIR /go/src/github.com/Tuananh165art/GoshopX
COPY go.mod go.sum ./
RUN go mod download
COPY admin admin
COPY pkg pkg
RUN GO111MODULE=on go build -mod=mod -o /go/bin/app ./admin/cmd/admin

FROM alpine:3.20
WORKDIR /usr/bin
COPY --from=build /go/bin/app ./app
EXPOSE 8080
CMD ["./app"]
