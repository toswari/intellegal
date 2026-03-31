#!/usr/bin/env sh
set -eu

ACTION="${1:-up}"
MIGRATIONS_DIR="infra/migrations"
PSQL_BASE="docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U ${POSTGRES_USER:-app} -d ${POSTGRES_DB:-legal_doc_intel}"

ensure_table() {
  sh -c "$PSQL_BASE" <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
  version BIGINT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SQL
}

applied_versions() {
  sh -c "$PSQL_BASE -t -A" <<'SQL'
SELECT version FROM schema_migrations ORDER BY version;
SQL
}

apply_file() {
  file="$1"
  cat "$file" | sh -c "$PSQL_BASE"
}

normalize_version() {
  version="$1"
  normalized=$(printf '%s' "$version" | sed 's/^0*//')
  if [ -z "$normalized" ]; then
    normalized=0
  fi
  printf '%s\n' "$normalized"
}

version_glob() {
  version="$1"
  printf '%04d\n' "$version"
}

ensure_table

case "$ACTION" in
  up)
    for file in $(ls "$MIGRATIONS_DIR"/*.up.sql 2>/dev/null | sort); do
      base=$(basename "$file")
      version=$(normalize_version "${base%%_*}")
      if applied_versions | grep -qx "$version"; then
        echo "Skipping $base (already applied)"
        continue
      fi

      echo "Applying $base"
      apply_file "$file"
      sh -c "$PSQL_BASE" <<SQL
INSERT INTO schema_migrations (version) VALUES ($version);
SQL
    done
    ;;
  down)
    version=$(sh -c "$PSQL_BASE -t -A" <<'SQL'
SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1;
SQL
)
    if [ -z "$version" ]; then
      echo "No applied migrations to roll back"
      exit 0
    fi

      version_prefix=$(version_glob "$version")
      file=$(ls "$MIGRATIONS_DIR"/"$version_prefix"_*.down.sql 2>/dev/null | head -n 1 || true)
    if [ -z "$file" ]; then
      echo "Down migration for version $version not found"
      exit 1
    fi

    echo "Rolling back $(basename "$file")"
    apply_file "$file"
    sh -c "$PSQL_BASE" <<SQL
DELETE FROM schema_migrations WHERE version = $version;
SQL
    ;;
  version)
    sh -c "$PSQL_BASE" <<'SQL'
SELECT version, applied_at FROM schema_migrations ORDER BY version;
SQL
    ;;
  *)
    echo "Usage: $0 [up|down|version]"
    exit 1
    ;;
esac
