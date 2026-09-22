# Build stage
FROM golang:1.27-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
# CGO_ENABLED=0 builds a static binary, allowing us to run it in scratch or alpine
# GOOS=linux ensures we are building for Linux, which is what Cloud Run uses
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server .

# Run stage
# We use Alpine instead of scratch to have access to CA certificates (for outbound HTTPS)
# and a shell (useful for debugging, though scratch could also be used for minimal footprint)
FROM alpine:3.19

WORKDIR /root/

# Install CA certificates for TLS (important if the app makes outbound HTTPS calls)
# and tzdata for timezone configuration
RUN apk --no-cache add ca-certificates tzdata

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/server .

# Cloud Run respects the PORT environment variable, which defaults to 8080.
ENV PORT=8080

# Expose port (Documentation purposes, Cloud Run overrides this dynamically)
EXPOSE ${PORT}

# Command to run the executable
CMD ["./server"]
