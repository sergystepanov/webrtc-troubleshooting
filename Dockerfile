FROM golang:1.20-bullseye

# UPX
ARG UPX_VERSION=3.96
RUN apt-get update && \
    apt-get install -y xz-utils && \
    rm -rf /var/lib/apt/lists/*
ADD https://github.com/upx/upx/releases/download/v$UPX_VERSION/upx-$UPX_VERSION-amd64_linux.tar.xz /usr/local
RUN xz -d -c /usr/local/upx-$UPX_VERSION-amd64_linux.tar.xz | \
    tar -xOf - upx-$UPX_VERSION-amd64_linux/upx > /bin/upx && \
    chmod a+x /bin/upx

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -ldflags "-w -s" -o ./build/ ./cmd/w3t
RUN strip --strip-unneeded ./build/w3t && \
    upx --best --lzma ./build/w3t

# deploy
FROM gcr.io/distroless/static-debian11

WORKDIR /

COPY --from=0 ./app/build/w3t /w3t
USER nonroot:nonroot
EXPOSE 3000 4000/udp

CMD [ "/w3t" ]
