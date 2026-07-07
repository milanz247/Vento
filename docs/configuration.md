# Configuration

Vento configures itself from a `.env` file at the project root, read once at
startup. There is no YAML, no config classes, no environment-specific file
sets — one flat file of `KEY=VALUE` pairs, and one function that turns the
database keys into a DSN. Both live in `vento/config.go` (about 70 lines —
worth reading in full).

## `.env` format

`vento.LoadEnv(path)` parses simple `KEY=VALUE` lines:

```dotenv
# Comments and blank lines are ignored.
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=root
DB_PASSWORD="a value with spaces"
DB_NAME=vento_app
```

Parsing rules, exactly as implemented:

- Lines starting with `#`, and blank lines, are skipped.
- The **first** `=` splits key from value (values may contain `=`).
- Keys and values are trimmed of surrounding whitespace; surrounding
  single or double quotes on the value are stripped.
- Every parsed pair is **also exported into the process environment** via
  `os.Setenv` — application code can read config through the returned
  `map[string]string` or plain `os.Getenv`, interchangeably.
- **A missing file is not an error.** `LoadEnv` returns an empty map and
  moves on. This is deliberate: in containers and CI, configuration
  usually arrives as real environment variables with no `.env` file at
  all. (Note the current demo reads DB keys from the returned map, so
  when running file-less you'd pass values by other means or adapt
  `main.go` to fall back to `os.Getenv`.)

What it does **not** do — no variable interpolation (`${OTHER}`), no
multi-line values, no type coercion. Keep values simple.

## Building the MySQL DSN

```go
dsn, ok := vento.BuildMySQLDSN(env)
```

Assembles the string `gorm.io/driver/mysql` expects:

```
user:password@tcp(host:port)/name?charset=utf8mb4&parseTime=True&loc=Local
```

The hardcoded query parameters matter: `parseTime=True` makes MySQL
`DATETIME`/`TIMESTAMP` columns scan into Go `time.Time` (GORM requires
this), `charset=utf8mb4` gets real Unicode including emoji, and
`loc=Local` interprets times in the server's timezone.

`ok` is `false` when `DB_HOST`, `DB_USER`, or `DB_NAME` is missing — and
callers are expected to **abort startup loudly**:

```go
if !ok {
    log.Fatal("vento: DB_HOST/DB_USER/DB_NAME missing from .env - MySQL configuration is required")
}
```

A server that boots half-configured and 500s on every query is strictly
worse than one that refuses to start with a clear message. This principle
runs through the whole boot — see
[Bootstrapping](bootstrapping.md#boot-order-rules-summarized).

Because the DSN is assembled at runtime from discrete keys, **no raw DSN
string ever exists in code or config** — credentials can't leak through a
committed connection string.

## Key reference

| Key | Required | Default |
|---|---|---|
| `DB_HOST` | Yes | — startup aborts if missing |
| `DB_USER` | Yes | — startup aborts if missing |
| `DB_NAME` | Yes | — startup aborts if missing |
| `DB_PORT` | No | `3306` |
| `DB_PASSWORD` | No | `""` (empty password) |

Adding your own application keys is free — anything in `.env` lands in the
returned map and the process environment. `SESSION_TTL=3600` is readable as
`env["SESSION_TTL"]` or `os.Getenv("SESSION_TTL")` with no framework
changes.

## Environment hygiene

- `.env` is gitignored. Keep it that way — never commit a file holding
  production credentials, and keep the repo's local copy at
  development-only values.
- `.air.toml` excludes `.env` from hot-reload watching, so editing it does
  **not** restart a dev server — restart manually after changing database
  configuration.
- The CLI (`db:migrate`, `db:seed`) reads the same `./.env`, which is why
  it must run from the project root.

See [Security](security.md) for the broader threat model this feeds into.
