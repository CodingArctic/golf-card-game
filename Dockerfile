# Multi-stage Dockerfile for Golf Card Game
# Stage 1: Build Next.js frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy package files and install dependencies
COPY frontend/package*.json ./
RUN npm ci

# Copy frontend source code
COPY frontend/ ./

# Build arguments for Next.js environment variables
ARG NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_TURNSTILE_SITE_KEY

# Create .env file for build
RUN echo "NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}" > .env && \
    echo "NEXT_PUBLIC_TURNSTILE_SITE_KEY=${NEXT_PUBLIC_TURNSTILE_SITE_KEY}" >> .env

# Build static export
RUN npm run build

# Stage 2: Build Go application
FROM golang:1.23-alpine AS go-builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o golf-card-game .

# Stage 3: Create minimal runtime image
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the compiled Go binary from builder
COPY --from=go-builder /app/golf-card-game .

# Copy the built Next.js static files from frontend builder
COPY --from=frontend-builder /app/frontend/out ./frontend/out

# Expose the application port (default 8080)
EXPOSE 8080

# Run the application
CMD ["./golf-card-game"]
