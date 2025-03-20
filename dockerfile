# Build stage
FROM golang AS builder

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o go_cmd_run .

# Runtime stage
FROM alpine

WORKDIR /app

# Copy binary from build stage
COPY --from=builder /app/go_cmd_run .

# Create config directory
RUN mkdir -p /app/config

# Copy static directory
COPY static/ /app/static/

# Set environment variables
ENV CONFIG_PATH=/app/config/config.json

# Expose port
EXPOSE 8080

# Run the application
CMD ["./go_cmd_run"]
