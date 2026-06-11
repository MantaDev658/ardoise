FROM oven/bun:1 AS frontend
WORKDIR /app
COPY apps/frontend/package.json apps/frontend/bun.lock ./
RUN bun install --frozen-lockfile
COPY apps/frontend/ .
RUN bun run build

FROM golang:1.26-alpine AS backend
WORKDIR /app
COPY go.work go.work.sum ./
COPY libs/ libs/
COPY apps/backend/ apps/backend/
RUN go build -ldflags="-s -w" -o /ardoise-api ./apps/backend/cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=backend /ardoise-api /ardoise-api
COPY --from=frontend /app/build /web
EXPOSE 8080
ENTRYPOINT ["/ardoise-api"]
