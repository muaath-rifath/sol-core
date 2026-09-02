.PHONY: up down test lint migrate

up:
	docker compose up --build

down:
	docker compose down

test:
	go -C apps/core test ./...
	go -C apps/builder test ./...
	pnpm --dir apps/web exec tsc --noEmit

lint:
	pnpm --dir apps/web lint

migrate:
	./scripts/migrate.sh
