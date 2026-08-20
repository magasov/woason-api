.PHONY: migrate migrate-down run test tidy

migrate:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

run:
	go run ./cmd/api

test:
	go test ./... -count=1

tidy:
	go mod tidy
