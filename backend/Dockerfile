FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s -extldflags '-static'" \
    -o /out/tashan-weir-server ./cmd/main.go

FROM scratch

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/tashan-weir-server /tashan-weir-server

COPY configs /configs
COPY ../frontend /frontend

EXPOSE 8080
EXPOSE 6060
EXPOSE 9090

ENTRYPOINT ["/tashan-weir-server"]
