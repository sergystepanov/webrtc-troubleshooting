# w3t (webrtc-troubleshooting)

A simple tool for troubleshooting WebRTC issues.

This utility contains a web server application designed for convenient debugging of problems with WebRTC connections.

### Use

Just run the w3t executable. Open the main page in the browser (i.e. localhost:3000).

Config params:

```
  -addr string
        a web server address (default ":3000")
```

### Build

Install Golang. Run:

```
go build ./cmd/w3t
```
