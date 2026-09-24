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
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/server .

# Run stage
# Distroless estático: traz certificados de CA e tzdata, e nada além disso — sem shell,
# sem gerenciador de pacotes. Era alpine:3.19, fora de suporte desde novembro de 2025,
# rodando como root. A tag `nonroot` roda com o usuário 65532.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/server .

# Cloud Run respects the PORT environment variable, which defaults to 8080.
ENV PORT=8080

# Expose port (Documentation purposes, Cloud Run overrides this dynamically)
EXPOSE ${PORT}

# Command to run the executable
CMD ["./server"]
