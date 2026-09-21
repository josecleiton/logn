#!/usr/bin/env bash
set -e

echo "Starting Docker containers (PostgreSQL and Mailpit)..."
docker compose up -d

echo "Waiting for PostgreSQL to be ready..."
sleep 3

echo "Starting Go API Server..."
cd backend
go run main.go auth_handlers.go otp_handlers.go progression_handlers.go &
GO_PID=$!

echo "Go API is running (PID: $GO_PID)."
echo "Press Ctrl+C to stop."

# Wait for process to exit
wait $GO_PID
