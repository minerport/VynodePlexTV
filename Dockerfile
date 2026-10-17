FROM node:24-alpine AS frontend
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS backend-base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal

FROM backend-base AS backend-test
RUN go test ./... && go vet ./... && test -z "$(gofmt -l cmd internal)"

FROM backend-base AS backend
ARG VERSION=0.2.0
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /server ./cmd/server

FROM alpine:3.23 AS runtime
RUN apk add --no-cache ca-certificates tzdata && addgroup -g 10001 app && adduser -D -u 10001 -G app app && mkdir -p /config && chown app:app /config
WORKDIR /app
COPY --from=backend /server /app/server
COPY --from=frontend /src/web/dist /app/web
ENV CONFIG_DIR=/config WEB_DIR=/app/web
USER 10001:10001
EXPOSE 8080
VOLUME /config
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s CMD wget -q -O /dev/null http://127.0.0.1:8080/api/v1/ready || exit 1
ENTRYPOINT ["/app/server"]
