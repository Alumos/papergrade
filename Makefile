.PHONY: test build web dev-api
# Use caller Go environment. In restricted workspaces set GOPATH/GOCACHE explicitly.
test:
	go test ./...
	cd web && pnpm run test
build:
	go build -trimpath -o bin/papergrade ./cmd/papergrade
	cd web && pnpm run build
web:
	cd web && pnpm run dev
dev-api:
	@echo 'Set ADMIN_USERNAME, ADMIN_PASSWORD and DEV_MODE=true first; memory dev is not durable.'
	go run ./cmd/papergrade api
