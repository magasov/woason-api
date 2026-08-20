FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/api /app/api
COPY migrations /app/migrations
RUN mkdir -p /app/uploads
ENV MIGRATIONS_PATH=/app/migrations
ENV UPLOAD_DIR=/app/uploads
EXPOSE 8080
CMD ["/app/api"]
