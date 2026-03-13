FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags "-s -w -X main.version=${VERSION}" \
    -o /sparkyfish-server ./cmd/sparkyfish-server

FROM alpine:latest
RUN addgroup -S sparkyfish && adduser -S sparkyfish -G sparkyfish
COPY --from=build /sparkyfish-server /sparkyfish-server
USER sparkyfish
EXPOSE 7121
ENTRYPOINT ["/sparkyfish-server"]
