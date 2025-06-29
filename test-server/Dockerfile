FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY main.go .
RUN go mod init test-server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"] 