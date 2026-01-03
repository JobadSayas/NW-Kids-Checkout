KIDS_CHECKIN_DB_FILE := kids-checkin.db

BIN_NAME := kids-checkin
BIN_PATH := ./bin/$(BIN_NAME)

# help must be first so that it is the default.
.PHONY: help
help:
	@grep -vE '^(\.PHONY|.*:=)' Makefile | grep '^[^#[:space:]].*:' | cut -d: -f1

.PHONY: db-reset
db-reset:
	rm -f $(KIDS_CHECKIN_DB_FILE) && \
    touch $(KIDS_CHECKIN_DB_FILE) && \
    migrate -source file://db/migrations -database "sqlite3://$(KIDS_CHECKIN_DB_FILE)" up

.PHONY: db-migrate
db-migrate:
	migrate -source file://db/migrations -database "sqlite3://$(KIDS_CHECKIN_DB_FILE)" up && \
	sqlite3 $(KIDS_CHECKIN_DB_FILE) .schema > db/structure.sql

# usage: make db-new-migration NAME=<migration name>
.PHONY: db-new-migration
db-new-migration:
	@if [ -z "$(NAME)" ]; then \
		echo "ERROR: You must provide a migration name using NAME=."; \
		exit 1; \
	fi
	@echo "Creating new migration: $(NAME)..."
	migrate create -ext sqlite -dir db/migrations $(NAME)

.PHONY: build
build:
	mkdir -pv bin && \
    godotenv go build -o $(BIN_PATH) main.go

.PHONY: web
web: build
	godotenv $(BIN_PATH) apiserver

.PHONY: checkout-fetcher
checkout-fetcher: build
	godotenv $(BIN_PATH) checkout-fetcher

.PHONY: test
test:
	godotenv go test ./...

.PHONY: db-seed
db-seed:
	godotenv ./bin/csv-import