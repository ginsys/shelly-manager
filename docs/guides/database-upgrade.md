# Upgrading an existing database

Shelly Manager repairs legacy schemas at startup, before GORM's AutoMigrate runs.
AutoMigrate cannot add a `NOT NULL` column to a table that already holds rows —
SQLite refuses the `ALTER` outright and PostgreSQL rejects the NULLs it would
create — so the server would otherwise abort on upgrade:

```
ALTER TABLE `config_templates` ADD `scope` text NOT NULL
→ SQL logic error: Cannot add a NOT NULL column with default value NULL (1)
ERROR Database migration failed  models=15
ERROR Failed to initialize database
```

A fresh database was never affected: `CREATE TABLE` carries the constraints
itself. This only ever bit databases created before a column was added.

## What the startup check does

For `config_templates`:

1. **Preflight (read-only).** Every row is classified, and nothing is written.
   If any row cannot be resolved the server refuses to start and lists **all**
   offending templates at once.
2. **Data phase (one transaction).** Missing columns are added nullable and
   without a default, and each row is given the scope the preflight decided on.
3. **Constraint phase.** `scope` and `config` are tightened to `NOT NULL`, each
   only if it is actually still nullable.

The check is idempotent: on an already-migrated database it does nothing.

### How a missing scope is inferred

The scope is derived from the legacy `device_type` column, and the rule can only
preserve or narrow which devices a template applies to — never widen it:

| legacy `device_type` | resulting scope |
|---|---|
| exactly `all` | `global` |
| any other non-empty value (`SHSW-1`, `SHPLG-S`, …) | `device_type` |
| empty or `NULL` | **startup aborts** |

`all` was the only wildcard the legacy matcher understood, and it matched
case-sensitively — so `All` is treated as a concrete device type, not a
wildcard. An empty `device_type` matched no device at all; mapping it to
`global` would silently turn an unusable template into a fleet-wide one, which
is why it is reported instead of guessed.

## When startup refuses

The error names every row that needs attention, with the reason:

```
config_templates cannot be migrated: 2 template(s) need manual resolution before startup
  - id=3 name="legacy-template": scope is missing and device_type is empty, so the intended scope is ambiguous
  - id=7 name="broken": config is NULL, but the schema requires a value
```

Resolve each row and restart. The database is left untouched, so you can open it
immediately and repair it in place.

| Reported reason | What to do |
|---|---|
| `device_type is empty` | Set `device_type` to `all` for a fleet-wide template, or to the device type the template was written for. |
| `config is NULL` | Give the template a config document, or delete the row. It is never filled in with `{}` automatically — fabricating a template body is worse than refusing to guess. |
| `scope "…" is invalid` | The scope must be `global`, `group` or `device_type`. Older API clients could store anything, including an empty string. |
| `device_type required when scope is 'device_type'` | Give the template a concrete `device_type`, or change its scope. |
| `config column is missing` | The column cannot be reconstructed. Restore from a backup, or drop the rows. |

Example repair with the SQLite CLI:

```sql
-- inspect what the migration is complaining about
SELECT id, name, device_type, config FROM config_templates;

-- a template meant for every device
UPDATE config_templates SET device_type = 'all' WHERE id = 3;

-- a template meant for one device type
UPDATE config_templates SET device_type = 'SHSW-1' WHERE id = 3;

-- or drop a row that is no longer wanted
DELETE FROM config_templates WHERE id = 7;
```

The API can no longer create these rows: `POST` and `PUT /api/v1/config/templates`
now reject an invalid scope with a `400` instead of storing it.

## What is guaranteed if something fails

The repair is not atomic as a whole — the constraint phase necessarily runs
after the data transaction commits:

- **Preflight failure** — nothing is written at all.
- **Data-phase failure** — SQLite and PostgreSQL roll the transaction back.
  MySQL commits DDL implicitly, so it may keep a nullable column.
- **Constraint-phase failure** — one constraint may already be applied while the
  other is not.
- **A later AutoMigrate failure** leaves the completed repair in place.

In every case the next startup resumes safely from wherever the previous one
stopped.

## Provider notes

- **SQLite** — tightening a constraint rebuilds the table (that is how SQLite
  works). Indexes the application declares are recreated immediately afterwards;
  indexes you added by hand are not, so re-create them after an upgrade that
  changes constraints.
- **PostgreSQL / MySQL** — connecting to either was broken before this release
  (`failed to ping database: not connected to database`), so there are no
  long-lived installations to upgrade. MySQL additionally could not migrate the
  schema at all: `TEXT` columns cannot carry a `DEFAULT`, and indexed string
  columns need a bounded length. Both are fixed; indexed string columns are now
  `varchar(191)`, the largest utf8mb4 prefix that fits MySQL's 767-byte index
  limit.

## Verifying an upgrade

```bash
# point the server at the database explicitly — see issue #276: the
# database.path setting is currently ignored because the database.dsn default
# always wins, so a mis-set path silently uses data/shelly.db instead
SHELLY_DATABASE_DSN=/path/to/shelly.db ./shelly-manager server
```

A successful repair logs one line with what it did:

```
Repaired legacy config_templates schema  scope_column_added=true config_column_added=false
  rows_backfilled_global=2 rows_backfilled_device_type=5
  scope_constraint_tightened=true config_constraint_tightened=true
```
