# AGENTS.md - Guide for Agentic Coding Agents

## Build & Test Commands

- **Build**: `make build` or `go build -o scrapegoat cmd/scraper/main.go`
- **Test all**: `make test` or `go test -v -race -coverprofile=coverage.out ./...`
- **Test single package**: `go test -v ./internal/scraper`
- **Test single function**: `go test -v -run TestFunctionName ./path/to/package`
- **Format code**: `make fmt` or `go fmt ./...`
- **Lint**: `make lint` (requires golangci-lint) or `go vet ./...`
- **Full check**: `make all` (fmt + vet + test + build)

## Code Style Guidelines

### Imports
- Standard library imports first, then blank line, then third-party packages, then blank line, then local packages
- Example order: `encoding/json`, `fmt` → blank → `github.com/spf13/cobra` → blank → `github.com/tedkulp/scrapegoat/internal/...`

### Formatting & Structure
- Use `gofmt` for all formatting (tabs for indentation, automatic spacing)
- Package-level constants use UPPER_CASE or camelCase depending on visibility
- Exported types, functions, and methods must have doc comments starting with the name (e.g., `// Client handles...`)

### Naming Conventions
- Exported: `PascalCase` (e.g., `GetGameInfo`, `NewClient`)
- Unexported: `camelCase` (e.g., `apiBaseURL`, `parseRating`)
- Receivers: short 1-2 letter lowercase (e.g., `c *Client`, `g *Generator`)
- Avoid stuttering: use `client.New()` not `client.NewClient()` when package name provides context

### Types & Error Handling
- Return errors as last return value (e.g., `func Foo() (*Result, error)`)
- Wrap errors with context: `fmt.Errorf("failed to fetch data: %w", err)`
- Use named return values sparingly, primarily for clarity in complex functions
- Prefer struct initialization with field names: `Client{httpClient: ..., devID: ...}`

### Comments
- Do NOT add comments unless specifically requested - let the code speak for itself
- Exception: Exported functions/types MUST have doc comments

### Patterns in This Codebase
- Config loading uses Viper with YAML
- HTTP client uses 30s timeout, retry logic with exponential backoff (2s, 4s, 8s)
- File paths use `filepath` package, always handle both absolute and relative paths
- Media downloads use worker pool pattern with channels
- Cache uses JSON files in `~/.scrapegoat/` with MD5-based filenames
- ZIP files are extracted in-memory for hashing (see `internal/scraper/hasher.go:72-144`)
- Prefer region/language fallback patterns (see `internal/scraper/types.go:103-190`)
