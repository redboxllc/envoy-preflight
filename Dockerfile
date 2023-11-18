ARG GO_VERSION=1
FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:${GO_VERSION}-bookworm AS build

ARG VERSION=local
ARG TARGETOS
ARG TARGETARCH

COPY . /app
WORKDIR /app
RUN go get -d
RUN go test -test.timeout 50s 
RUN CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH" go build -o scuttle -ldflags="-X 'main.Version=${VERSION}'"

FROM scratch
COPY --from=build /app/scuttle /scuttle
