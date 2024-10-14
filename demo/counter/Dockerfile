FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o counter ./demo/counter/

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/counter .
ENTRYPOINT ["./counter"]
