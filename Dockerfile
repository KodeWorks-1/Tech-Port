FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /techport ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /techport ./techport
COPY views ./views
COPY static ./static
RUN mkdir -p uploads && chown -R app:app /app
USER app
EXPOSE 8080
ENTRYPOINT ["./techport"]
