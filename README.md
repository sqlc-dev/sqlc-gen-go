## Usage

> [!IMPORTANT]  
> This repository is a read-only portion of https://github.com/sqlc-dev/sqlc.

```yaml
version: '2'
plugins:
- name: golang
  wasm:
    url: "https://example.com"
    sha256: ""
sql:
- schema: schema.sql
  queries: query.sql
  engine: postgresql
  codegen:
  - plugin: golang
    out: db
    options:
      package: db
      sql_package: pgx/v5
```