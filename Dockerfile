FROM golang:alpine

RUN apk add git

ENV GOBIN=/out

COPY ./ /src

WORKDIR /src
RUN mkdir -p $GOBIN
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags='-w -s -extldflags "-static"' -o $GOBIN