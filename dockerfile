FROM golang:1.20-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o excel-to-postgres ./cmd



FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/excel-to-postgres .
COPY config/config.yaml .
EXPOSE 8080
CMD ["./excel-to-postgres"]
