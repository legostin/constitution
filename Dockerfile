FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /constitution ./cmd/constitution
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /constitutiond ./cmd/constitutiond

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /constitution /usr/local/bin/constitution
COPY --from=builder /constitutiond /usr/local/bin/constitutiond
EXPOSE 8081
ENTRYPOINT ["constitutiond"]
CMD ["--config", "/etc/constitution/config.yaml", "--addr", ":8081"]
