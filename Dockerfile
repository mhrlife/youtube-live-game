FROM golang:1.23.1-bullseye AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files to the working directory
COPY go.* ./

RUN go env -w GOPROXY=https://goproxy.cn,direct
# Download and cache dependencies
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go application
RUN CGO_ENABLED=0  go build -o /app/streamer main.go

# Use a lightweight image for the runtime environment
FROM ubuntu:20.04

# Install FFmpeg
RUN apt-get update && apt-get install -y ffmpeg && rm -rf /var/lib/apt/lists/*

# Set up the working directory for the runtime container
WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
# Copy the built Go application from the builder stage
COPY --from=builder /app/streamer .
COPY ./assets /app/assets
# Expose any ports (if needed)
# EXPOSE 8080 (uncomment if your application requires an exposed port)

# Run the Go application
CMD ["./streamer"]
