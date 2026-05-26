FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY libs/ libs/
COPY apps/backend/ apps/backend/
RUN go build -ldflags="-s -w" -o /ardoise-api ./apps/backend/cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /ardoise-api /ardoise-api
EXPOSE 8080
ENTRYPOINT ["/ardoise-api"]
