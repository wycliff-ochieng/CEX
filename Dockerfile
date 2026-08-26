FROM golang:1.21-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
# We use CGO_ENABLED=0 to ensure a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cex_engine ./cmd/...

# Start a new stage from scratch for a tiny footprint
FROM alpine:latest  

WORKDIR /root/

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/cex_engine .

# Expose port (The API runs on 8888)
EXPOSE 8888

# Command to run the executable
CMD ["./cex_engine"]
