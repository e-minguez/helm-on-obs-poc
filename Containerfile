#!BuildTag: hello-world-go:latest
FROM registry.opensuse.org/opensuse/bci/golang:latest AS build
WORKDIR /src
COPY app/ ./
ENV CGO_ENABLED=0 GOPROXY=off
RUN go build -o /hello-world .

FROM registry.opensuse.org/opensuse/busybox:latest
COPY --from=build /hello-world /hello-world
EXPOSE 8080
USER 1000
ENTRYPOINT ["/hello-world"]
