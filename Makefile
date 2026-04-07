up:
	docker compose --env-file .env -f deploy/docker-compose.yml up -d

down:
	docker compose -f deploy/docker-compose.yml down

restart:
	docker compose -f deploy/docker-compose.yml restart

logs:
	docker compose -f deploy/docker-compose.yml logs -f

run:
	go run ./cmd/server/main.go
