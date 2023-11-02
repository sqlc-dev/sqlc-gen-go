## Usage

> [!IMPORTANT]  
> This repository is read-only. It contains a working Go codegen plugin extracted from https://github.com/sqlc-dev/sqlc which you can fork and modify to meet your needs.

```yaml
version: '2'
plugins:
- name: golang
  wasm:
    url: https://downloads.sqlc.dev/plugin/sqlc-gen-go_1.0.0.wasm
    sha256: dbe302a0208afd31118fffcc268bd39b295655dfa9e3f385d2f4413544cfbed1
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
