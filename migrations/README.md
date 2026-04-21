# Migrations

This directory holds **PostgreSQL** schema files (`*.up.sql`). They are mounted
into the Postgres container by `docker-compose.yml` as
`docker-entrypoint-initdb.d`, so a fresh `docker compose --profile full up`
will create the schema automatically the first time the database is
initialized.

## Current status (read this first)

The agent's Go code currently runs against the **embedded SQLite** backend
(`modernc.org/sqlite`, default `data/yunque.db`), with the application-level
data layer using `Ledger KV` over SQLite. There is **no `database/sql`
PostgreSQL driver imported by the agent today** — these `.up.sql` files are
provisioned for the eventual Postgres data path and to support
operational tools that connect directly to the database (psql, BI, etc.).

In other words:

* Setting `DATABASE_URL` does **not** today route the agent to PostgreSQL.
  The variable is read by the setup wizard and docker-compose, but the
  runtime always opens SQLite.
* Running `docker compose --profile full up` will create the Postgres
  container and apply these migrations, but the agent inside the same
  compose still only writes to its embedded SQLite under `/app/data`.

When the agent learns to talk to Postgres, the wiring will need:

1. A Go driver (`github.com/jackc/pgx/v5/stdlib` or `github.com/lib/pq`).
2. A migration runner (or a documented `psql -f` step) that applies
   these files in order against the configured `DATABASE_URL`.
3. `internal/storage/sqlite.go` (or its successor) abstracted behind a
   driver-agnostic interface so the rest of the codebase does not have to
   know which backend is active.

## Conventions

* `NNN_name.up.sql` — forward migration, executed in numeric order.
* `pgvector` and `uuid-ossp` extensions are required (created on first run).
* No `.down.sql` files yet; treat schema changes as additive when possible
  and document any destructive change in the file header.
