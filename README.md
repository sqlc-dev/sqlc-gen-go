## Usage

> [!IMPORTANT]  
> This repository is read-only. It contains a working Go codegen plugin extracted from https://github.com/sqlc-dev/sqlc which you can fork and modify to meet your needs.

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