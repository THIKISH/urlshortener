# syntax=docker/dockerfile:1

FROM golang:1.21-alpine AS build

WORKDIR /app

# Copy the source code
COPY main.go ./
COPY index.html ./

# Build the application
RUN go build -o url-shortener main.go

# Use a lightweight alpine image for the runtime
FROM alpine:latest

WORKDIR /app

# Copy the binary and static files from the build stage
COPY --from=build /app/url-shortener .
COPY --from=build /app/index.html .

# Expose the port the app runs on
EXPOSE 8080

# Command to run the executable
CMD ["./url-shortener"]
