# 🚀 Mockly

**Zero-configuration mock data generator and REST API server**

[![CI Pipeline](https:///actions/workflows/ci.yml/badge.svg)](https:///actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/)](https://goreportcard.com/report/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26+-blue.svg)](https://go.dev/)

Mockly generates realistic mock data based on a simple JSON schema and serves it over a REST API — no databases to install, no migrations to run, no complex configuration.

```bash
# Define your schema → Start the server → Done
mockly up
curl http://localhost:3000/api/users
```

---

## ✨ Features

- **📝 Define once, run anywhere** — Describe your data schema in JSON, Mockly handles the rest
- **🧠 Smart data generation** — 15+ data types: UUIDs, names, emails, addresses, timestamps, and more
- **⚡ Zero configuration** — No databases, no ORMs, no migrations. Just one binary and one config file
- **🔄 Smart Wipe** — Automatically detects changes and reseeds data without unnecessary operations
- **🔌 REST API** — Full CRUD endpoints auto-generated from your schema
- **📦 Chunked seeding** — Handles thousands of rows efficiently with batch inserts
- **🔍 Paginated queries** — Use `?limit=100&offset=0` for large datasets
- **🔒 Security-first** — SQL injection safeguards, path traversal prevention, and strict validation

---

## 📋 Table of Contents

- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [CLI Usage](#cli-usage)
- [API Endpoints](#api-endpoints)
- [Supported Data Types](#supported-data-types)
- [Project Structure](#project-structure)
- [Contributing](#contributing)
- [Security](#security)
- [License](#license)

---

## ⚡ Quick Start

### Installation

```bash
# Clone the repository
git clone https://.git
cd mockly

# Build the binary
go build -o mockly.exe ./cmd/mockly
```

### Run the server

Create a `mockly.json` file:

```json
{
  "seed_count": 50,
  "schema": {
    "users": {
      "id": { "type": "uuid" },
      "name": { "type": "name" },
      "email": { "type": "email" },
      "created_at": { "type": "timestamp" }
    }
  }
}
```

Then start the server:

```bash
# Start with default configuration
mockly up

# Start on a custom port
mockly up --port 8080

# Seed more records
mockly up --seed 500
```

That's it! Your API is now live at `http://localhost:3000`.

```bash
# Try it out
curl http://localhost:3000/api/users
curl http://localhost:3000/api/health
```

---

## 📝 Configuration

### `mockly.json`

| Field | Type | Description |
|-------|------|-------------|
| `seed_count` | integer | Number of mock records to generate per table |
| `schema` | object | Map of table names to field definitions |

### Schema Field Types

See the [Supported Data Types](#supported-data-types) section below.

### Example: Multiple Tables

```json
{
  "seed_count": 100,
  "schema": {
    "users": {
      "id": { "type": "uuid" },
      "name": { "type": "name" },
      "email": { "type": "email" },
      "created_at": { "type": "timestamp" }
    },
    "products": {
      "id": { "type": "uuid" },
      "name": { "type": "text" },
      "price": { "type": "float" },
      "in_stock": { "type": "boolean" },
      "category_id": { "type": "number" }
    },
    "orders": {
      "id": { "type": "uuid" },
      "user_id": { "type": "uuid" },
      "total": { "type": "float" },
      "status": { "type": "text" },
      "ordered_at": { "type": "timestamp" }
    }
  }
}
```

---

## 🔧 CLI Usage

```bash
mockly up        # Start the mock API server
mockly reset     # Wipe the database and reseed with fresh data
```

### Flags for `up` and `reset`

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `3000` | Port to run the API server on |
| `--db` | `mockly.db` | Path to the SQLite database file |
| `--config` | (auto-detect) | Path to `mockly.json` config file |
| `--seed` | (from config) | Override seed count from config |

### Examples

```bash
# Development server on custom port
mockly up --port 8080

# Large dataset for load testing
mockly up --seed 10000

# Custom config and database paths
mockly up --config ./examples/ecommerce.json --db ./data/test.db

# Reset and reseed
mockly reset --seed 200
```

---

## 🌐 API Endpoints

Mockly automatically creates endpoints for each table in your schema.

### Auto-generated Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/{table}` | List all records (supports `?limit=N&offset=N`) |
| `GET` | `/api/{table}/{id}` | Get a single record by ID |
| `POST` | `/api/{table}` | Create a new record with generated data |
| `DELETE` | `/api/{table}/{id}` | Delete a record by ID |
| `GET` | `/api/health` | Health check endpoint |

### Pagination

Query large tables efficiently with pagination:

```bash
# Get first 100 records
curl "http://localhost:3000/api/users?limit=100&offset=0"

# Get next page
curl "http://localhost:3000/api/users?limit=100&offset=100"
```

---

## 🎲 Supported Data Types

| Type | Description | Example Output |
|------|-------------|----------------|
| `uuid` | UUID v4 | `"550e8400-e29b-41d4-a716-446655440000"` |
| `name` | Full name | `"John Smith"` |
| `email` | Email address | `"jane.doe@example.com"` |
| `timestamp` | ISO 8601 timestamp | `"2024-01-15T10:30:00Z"` |
| `phone` | Phone number | `"+1-555-123-4567"` |
| `address` | Street address | `"123 Main Street"` |
| `city` | City name | `"San Francisco"` |
| `country` | Country name | `"United States"` |
| `number` | Integer | `42` |
| `float` | Float number | `19.99` |
| `boolean` | True/false | `true` |
| `text` | Sentence text | `"Lorem ipsum dolor sit amet..."` |
| `url` | URL | `"https://example.com/page"` |
| `username` | Username | `"johndoe_42"` |
| `password` | Secure password | `"aB3$kL9#xP2m"` |

---

## 🏗️ Project Structure

```
mockly/
├── cmd/
│   └── mockly/
│       └── main.go              # Entry point & CLI logic
├── internal/
│   ├── config/
│   │   └── parser.go            # Config loading, validation & sanitization
│   ├── database/
│   │   └── sqlite.go            # SQLite operations, connection pool, indexes
│   ├── faker/
│   │   └── generator.go         # Mock data generation engine
│   └── server/
│       └── router.go            # HTTP routing, handlers & pagination
├── .github/
│   ├── workflows/
│   │   └── ci.yml               # CI pipeline with tests, lint & security scan
│   └── ISSUE_TEMPLATE/          # Bug report & feature request templates
├── CONTRIBUTING.md              # Contribution guide
├── SECURITY.md                  # Security policy
└── LICENSE                      # MIT License
```

### Architecture Principles

- **Decoupled**: Each `internal/` package has a single responsibility
- **Self-documenting**: Short functions with descriptive names
- **Standard library only**: Uses Go's built-in `net/http` (Go 1.22+ routing patterns)
- **Testable**: Clean interfaces make unit testing straightforward

---

## 🤝 Contributing

We welcome contributions! Please read our [Contributing Guide](CONTRIBUTING.md) for details on:

- Setting up your development environment
- Code style and conventions
- Testing guidelines
- Pull request process

### Quick Start for Contributors

```bash
# Clone and build
git clone https://.git
cd mockly
go build -o mockly.exe ./cmd/mockly

# Run tests
go test -v -race ./...

# Run linter
golangci-lint run ./...
```

### Development Roadmap

- [ ] Relationship-aware foreign key generation
- [ ] Custom data generators (plugins)
- [ ] OpenAPI/Swagger documentation generation
- [ ] GraphQL support
- [ ] Export to CSV/JSON files
- [ ] Web UI for schema management

---

## 🛡️ Security

We take security seriously. See our [Security Policy](SECURITY.md) for:

- 🔒 How to report vulnerabilities privately
- ✅ Supported versions
- 🛡️ Security best practices

### Built-in Protections

- **SQL Injection Prevention**: Table and field names are validated against strict alphanumeric regex patterns
- **Path Traversal Prevention**: All file paths are sanitized with `filepath.Clean()`
- **Input Validation**: Schema types are validated against a whitelist of supported types
- **Connection Pool Limits**: Database connections are bounded to prevent resource exhaustion

---

## 📄 License

Mockly is open source software [licensed as MIT](LICENSE). You are free to use, modify, and distribute it as part of your projects.

---

## 🙏 Acknowledgments

- [gofakeit](https://github.com/brianvoe/gofakeit) — Fake data generation library
- [modernc.org/sqlite](https://modernc.org/sqlite) — Pure Go SQLite implementation
- All [contributors](https://github.com/poria-lang/mockly/graphs/contributors) who help improve Mockly

---

<p align="center">Made with ❤️ for developers who need mock data fast</p>