# TechPort dev commands

# start local postgres
db-up:
    docker compose up -d db

# stop local postgres
db-down:
    docker compose down

# run the server (migrates + seeds automatically)
dev:
    go run ./cmd/server

# build production binary
build:
    go build -ldflags "-s -w" -o bin/techport.exe ./cmd/server

test:
    go test ./...
