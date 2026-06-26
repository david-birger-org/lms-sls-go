MIGRATIONS_DIR ?= db/migrations
DOWN_MIGRATIONS_DIR ?= db/migrations/down

.PHONY: migrate-up migrate-down

migrate-up:
	@if [ -z "$$DATABASE_URL_DIRECT" ]; then \
		echo "DATABASE_URL_DIRECT is missing; use the direct Neon/Postgres URL for migrations." >&2; \
		exit 1; \
	fi
	@set -eu; \
	found=0; \
	for file in $(MIGRATIONS_DIR)/*.sql; do \
		if [ ! -e "$$file" ]; then \
			break; \
		fi; \
		found=1; \
		echo "Applying $$file"; \
		psql "$$DATABASE_URL_DIRECT" -v ON_ERROR_STOP=1 -f "$$file"; \
	done; \
	if [ "$$found" -eq 0 ]; then \
		echo "No SQL migration files found in $(MIGRATIONS_DIR)." >&2; \
		exit 1; \
	fi

migrate-down:
	@if [ -z "$$DATABASE_URL_DIRECT" ]; then \
		echo "DATABASE_URL_DIRECT is missing; use the direct Neon/Postgres URL for migrations." >&2; \
		exit 1; \
	fi
	@set -eu; \
	files=$$(find "$(DOWN_MIGRATIONS_DIR)" -maxdepth 1 -type f -name "*.sql" 2>/dev/null | sort -r); \
	if [ -z "$$files" ]; then \
		echo "No down migrations found in $(DOWN_MIGRATIONS_DIR)." >&2; \
		echo "Create explicit down migrations before using migrate-down." >&2; \
		exit 1; \
	fi; \
	for file in $$files; do \
		echo "Applying $$file"; \
		psql "$$DATABASE_URL_DIRECT" -v ON_ERROR_STOP=1 -f "$$file"; \
	done
