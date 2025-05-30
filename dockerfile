# Build stage
FROM golang:latest AS builder

ENV HTTP_PROXY=http://clash:7890
ENV HTTPS_PROXY=http://clash:7890
ENV NO_PROXY=localhost,127.0.0.1

WORKDIR /app

# Copy go mod files first to leverage Docker cache
COPY go.mod ./
# 如果存在 go.sum 则复制
COPY go.sum* ./

# 显式添加 websocket 依赖
RUN go get github.com/gorilla/websocket
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o go_cmd_run .

# Runtime stage
FROM alpine:latest

WORKDIR /app

ENV PATH="${PATH}:/app/scripts"

# Copy the binary from builder
COPY --from=builder /app/go_cmd_run .

# Copy static files if needed
COPY static/ ./static/

# Expose the port
EXPOSE 8080

# Run the application
CMD ["./go_cmd_run"]
