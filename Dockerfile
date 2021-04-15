FROM golang:alpine AS builder

RUN apk update
RUN apk add --no-cache git
WORKDIR /app/

RUN go get github.com/pin/tftp

COPY main.go main.go
RUN CGO_ENABLED=0 go build -o /main

FROM alpine AS tftp 
RUN apk add --no-cache wget
RUN apk add --no-cache syslinux

FROM scratch
WORKDIR /files/
COPY initrfs.img .
COPY vmlinuz .
COPY slax .
COPY PXEFILELIST .
WORKDIR /tftp/
COPY --from=tftp /usr/share/syslinux/lpxelinux.0 .
COPY --from=tftp /usr/share/syslinux/ldlinux.c32 .
WORKDIR /tftp/pxelinux.cfg/
WORKDIR /tftp/
COPY --from=builder /main /tftp/main
ENTRYPOINT ["/tftp/main"]
