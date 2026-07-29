FROM golang:1.24-alpine3.20 AS build
RUN apk --no-cache add gcc g++ make ca-certificates
WORKDIR /go/src/github.com/Tuananh165art/GoshopX
COPY go.mod go.sum ./
RUN go mod download
COPY account account
COPY product product
COPY order order
COPY recommender recommender
COPY payment payment
COPY inventory inventory
COPY cart cart
COPY notification notification
COPY graphql graphql
COPY pkg pkg
RUN GO111MODULE=on go build -mod mod -o /go/bin/app ./graphql/cmd/graphql

FROM alpine:3.20
WORKDIR /usr/bin
COPY --from=build /go/bin .
EXPOSE 8080
CMD ["app"]
