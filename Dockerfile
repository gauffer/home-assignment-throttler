FROM golang:1.21 as builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -v -o simulator cmd/simulator/main.go

FROM debian:bullseye-slim

WORKDIR /root/

COPY --from=builder /app/simulator .

EXPOSE 8080

CMD ["./simulator"]
