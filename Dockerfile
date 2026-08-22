FROM golang:1.23-alpine AS builder

WORKDIR /app
RUN apk add --no-cache ca-certificates git
ENV CGO_ENABLED=0 GOOS=linux

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-w -s" -o bot .

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata \
    && addgroup -S bot \
    && adduser -S -G bot bot

WORKDIR /app
COPY --from=builder /app/bot .
ENV TZ=Asia/Baghdad
USER bot
EXPOSE 8000

CMD ["./bot"]
