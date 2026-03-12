FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod .
COPY main.go .
RUN go build -o victim-app .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/victim-app .
EXPOSE 9995
ENTRYPOINT ["./victim-app"]
