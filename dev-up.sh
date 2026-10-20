#!/usr/bin/env bash
set -euo pipefail

API_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$API_DIR"

if [[ ! -f .env ]]; then
    echo "Arquivo visionflow-api/.env não encontrado." >&2
    exit 1
fi

if [[ ! -f db/flyway.conf ]]; then
    echo "Arquivo db/flyway.conf não encontrado. Configure o Flyway antes de preparar o banco." >&2
    exit 1
fi

if ! command -v flyway >/dev/null 2>&1; then
    echo "Flyway não encontrado no PATH." >&2
    exit 1
fi

if ! command -v psql >/dev/null 2>&1 || ! command -v pg_isready >/dev/null 2>&1; then
    echo "psql e pg_isready são necessários; instale o cliente PostgreSQL local." >&2
    exit 1
fi

set -a
# The API's local dotenv file is shell-compatible and ignored by Git.
. ./.env
set +a

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:?DB_USER precisa estar definido em .env}"
DB_NAME="${DB_NAME:?DB_NAME precisa estar definido em .env}"
DB_PASSWORD="${DB_PASSWORD:?DB_PASSWORD precisa estar definido em .env}"
export PGPASSWORD="$DB_PASSWORD"
export FLYWAY_PASSWORD="$DB_PASSWORD"
export FLYWAY_POSTGRESQL_TRANSACTIONAL_LOCK=false

echo "Verificando PostgreSQL em ${DB_HOST}:${DB_PORT}..."
pg_isready -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER"

echo "Validando acesso ao banco ${DB_NAME}..."
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
    -X -v ON_ERROR_STOP=1 -c 'SELECT 1;' >/dev/null

echo "Aplicando migrations Flyway..."
flyway -configFiles=./db/flyway.conf migrate

echo "Aplicando seeds idempotentes do Demo Comercial..."
./db/seeds/demo_comercial/apply_all.sh

echo "Banco local preparado. O Playwright inicia a API e o Vite ao executar os testes."
