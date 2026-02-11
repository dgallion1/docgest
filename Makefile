.PHONY: help up down build test secrets secrets-encrypt secrets-edit

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

## ---------------------------------------------------------------------------
## Secrets (SOPS + age)
## ---------------------------------------------------------------------------

secrets: ## Decrypt .env.enc → .env (if .env.enc is newer or .env is missing)
	@if [ -f .env.enc ]; then \
		if [ ! -f .env ] || [ .env.enc -nt .env ]; then \
			command -v sops >/dev/null 2>&1 || { echo "ERROR: sops not installed" >&2; exit 1; }; \
			sops --decrypt --input-type dotenv --output-type dotenv .env.enc > .env; \
			echo "  Decrypted .env.enc → .env"; \
		fi; \
	fi

secrets-encrypt: ## Encrypt .env → .env.enc
	@if [ -f .env ] && [ -f .sops.yaml ]; then \
		sops --encrypt --input-type dotenv --output-type dotenv .env > .env.enc; \
		echo "Encrypted .env → .env.enc"; \
	fi

secrets-edit: ## Edit secrets (decrypt in editor, re-encrypt on save)
	sops --input-type dotenv --output-type dotenv .env.enc

## ---------------------------------------------------------------------------
## Stack
## ---------------------------------------------------------------------------

up: secrets ## Start docgest + pathstore + postgres via Docker Compose
	docker compose --env-file .env up -d --build

down: ## Stop containers
	docker compose down

## ---------------------------------------------------------------------------
## Build & Test
## ---------------------------------------------------------------------------

build: ## Compile all packages
	go build ./...

test: ## Run unit tests
	go test ./...
