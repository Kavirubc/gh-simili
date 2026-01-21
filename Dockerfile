FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /gh-simili ./cmd/gh-simili

FROM alpine:3.19

RUN apk add --no-cache ca-certificates git

COPY --from=builder /gh-simili /usr/local/bin/gh-simili

ENTRYPOINT ["/usr/local/bin/gh-simili"]
