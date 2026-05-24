# Contributing to Mockly

Thank you for your interest in improving Mockly! To keep maintainer overhead sustainable, please follow this development path.

## 🛠️ Local Development Setup

1. **Prerequisites**: Ensure you have [Go 1.22+](https://go.dev) installed.
2. **Clone the repository**:
   ```bash
   git clone https://github.com
   cd mockly
   ```
3. **Verify tests pass locally**:
   ```bash
   go test -v -race ./...
   ```

## 📐 Development Rules

* **Formatting**: Run `go fmt ./...` before committing any code changes.
* **Architecture Rules**:
  * Do not add heavy third-party routing dependencies; stick to native `net/http` route matching.
  * Keep all dynamic table logic protected by the strict sanitization patterns in `internal/config/validator.go`.
* **Testing Requirement**: Every bug fix or new feature addition must be accompanied by corresponding test suites (`*_test.go`).

## 🚀 Creating a Pull Request

1. Create a branch focusing on your specific fix: `git checkout -b feature/your-feature-name`.
2. Commit clear, structural messages.
3. Push your branch and open a clean Pull Request targeting the `main` branch. Ensure the automated CI checks turn green!