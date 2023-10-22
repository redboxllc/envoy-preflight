FROM golang:1.21.3 AS build
ARG VERSION="local"

WORKDIR /app

# Download dependencies
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy actual go files
COPY *.go .

# Test and build
RUN go test -test.timeout 30s 
RUN CGO_ENABLED=0 go build -o scuttle -ldflags="-X 'main.Version=${VERSION}'"

FROM alpine as test
# This image is used for local testing scuttle in a container with shell tooling
COPY --from=build /app/scuttle /scuttle

FROM scratch
COPY --from=build /app/scuttle /scuttle
