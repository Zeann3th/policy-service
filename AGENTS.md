# Repository Rules

## Database migrations

- Create schema and data migrations as Goose SQL files with `goose create -s <subject> sql`.
- Keep database migration logic in SQL. Use SQL clients for database inspection and verification.
- Keep applied numbered migrations immutable; update `schema.sql` as the final-state schema snapshot.

