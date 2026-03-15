FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download && CGO_ENABLED=0 GOOS=linux go build -o /out/service ./cmd/main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /out/service /app/service
EXPOSE 8084
CMD ["/app/service"]
