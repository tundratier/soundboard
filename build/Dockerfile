# Cross-compile environment: Go + MinGW-w64 (for cgo/malgo → Windows exe)
FROM golang:1.27-bookworm
RUN apt-get update \
 && apt-get install -y --no-install-recommends gcc-mingw-w64-x86-64 \
 && rm -rf /var/lib/apt/lists/*
ENV CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc
WORKDIR /src/backend
