FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod .
COPY main.go .
RUN go build -o watchdog-app .

FROM alpine:3.19
# Docker CLI needed to run: docker start victim-b
RUN apk add --no-cache docker-cli
WORKDIR /app
COPY --from=builder /app/watchdog-app .
EXPOSE 9999
ENTRYPOINT ["./watchdog-app"]
