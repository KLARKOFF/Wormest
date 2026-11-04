FROM golang:1.26-alpine AS build
WORKDIR /src
# Сначала только go.mod/go.sum: слой с зависимостями кешируется между сборками
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.22
# poppler-utils (pdftotext) понадобится для извлечения текста из PDF
RUN apk add --no-cache ca-certificates poppler-utils \
 && adduser -D -H -u 10001 app \
 && mkdir -p /data/files && chown app /data/files
COPY --from=build /out/server /usr/local/bin/server
USER app
ENV STORAGE_DIR=/data/files HTTP_ADDR=:8080
EXPOSE 8080
CMD ["server"]
