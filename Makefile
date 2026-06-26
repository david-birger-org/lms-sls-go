GOOSE_VERSION ?= v3.27.1
GOOSE ?= go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)
GOOSE_DRIVER ?= postgres
GOOSE_TABLE ?= goose_db_version
MIGRATIONS_DIR ?= db/migrations

.PHONY: check-db-url migrate-up migrate-down migrate-status migrate-validate

check-db-url:
	@if [ -z "$$DATABASE_URL_DIRECT" ]; then \
		echo "DATABASE_URL_DIRECT is missing; use the direct Neon/Postgres URL for migrations." >&2; \
		exit 1; \
	fi

migrate-up: check-db-url
	@$(GOOSE) -dir "$(MIGRATIONS_DIR)" -table "$(GOOSE_TABLE)" $(GOOSE_DRIVER) "$$DATABASE_URL_DIRECT" up

migrate-down: check-db-url
	@$(GOOSE) -dir "$(MIGRATIONS_DIR)" -table "$(GOOSE_TABLE)" $(GOOSE_DRIVER) "$$DATABASE_URL_DIRECT" down

migrate-status: check-db-url
	@$(GOOSE) -dir "$(MIGRATIONS_DIR)" -table "$(GOOSE_TABLE)" $(GOOSE_DRIVER) "$$DATABASE_URL_DIRECT" status

migrate-validate:
	@$(GOOSE) -dir "$(MIGRATIONS_DIR)" validate
