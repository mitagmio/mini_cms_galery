FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates git
WORKDIR /app
COPY --from=build /out/server /app/server
RUN mkdir -p /data/uploads /data/preview
ENV LISTEN_ADDR=:8080 \
    DATA_DIR=/data \
    UPLOAD_DIR=/data/uploads \
    PREVIEW_DIR=/data/preview \
    FRONT_DIR=/front \
    FRONT_THEME_SRC=/front
EXPOSE 8080
CMD ["/app/server"]
