# syntax=docker/dockerfile:1

##
## Build
##
FROM golang:1.18-bullseye

WORKDIR /app

# install dependencies
# RUN apk add upx

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./
RUN go build -ldflags "-w -s" -o ./build/ ./cmd/w3t

##
## Deploy
##
FROM gcr.io/distroless/base-debian11

WORKDIR /

COPY --from=0 ./app/build/w3t /w3t
USER nonroot:nonroot
EXPOSE 3000 4000/udp

CMD [ "/w3t" ]
