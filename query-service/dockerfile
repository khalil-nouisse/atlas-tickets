# STAGE 1: Build the Go Binary
# We use the official Golang image to compile the code
FROM golang:1.24-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy dependency files first (to cache dependencies)
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
# -o main: names the binary "main"
# main.go: the entry point
RUN go build -o main main.go

# STAGE 2: Create the Tiny Runtime Image
FROM alpine:latest

# Install basic certificates (needed for HTTPS calls if you make any)
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy only the compiled binary from the builder stage
COPY --from=builder /app/main .

# Copy the .env file (Optional: usually better to set env vars in docker-compose)


# Expose the port your API runs on
EXPOSE 8081

# Command to run the executable
CMD ["./main"]