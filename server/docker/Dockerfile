# Service folder name inside ./cmd
ARG SERVICE_PATH=fear-online-server

##
## Stage: Base
##
##
FROM golang:1.17-alpine AS base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .


##
## Stage: Development
##
FROM base AS development
ARG SERVICE_PATH
RUN apk add --no-cache curl \
    && curl -sSfL https://raw.githubusercontent.com/air-verse/air/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin" \
    && sed "s/{SERVICE_PATH}/${SERVICE_PATH}/g" ./docker/air.toml > /root/.air.toml
EXPOSE 8080 30003
ENTRYPOINT ["air", "-c", "/root/.air.toml"]


##
## Stage: Build
##
FROM base AS builder
ARG SERVICE_PATH
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /go-service "./cmd/${SERVICE_PATH}"


##
## Stage: Release
##
FROM alpine:3.19 AS release
RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 fear
COPY --from=builder /go-service /usr/local/bin/go-service
USER fear
EXPOSE 8080 30003
ENTRYPOINT ["/usr/local/bin/go-service"]
