FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app app \
    && mkdir -p /app/logs \
    && chown -R app:app /app

WORKDIR /app
COPY --from=build /out/api ./api

USER app
EXPOSE 8080

ENTRYPOINT ["./api"]
