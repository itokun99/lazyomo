# Contributing to lazyomo

## Development Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/itokun99/lazyomo.git
   cd lazyomo
   ```

2. Install Go 1.22+

3. Build:
   ```bash
   go build -o lazyomo ./cmd/lazyomo
   ```

4. Run tests:
   ```bash
   go test ./...
   ```

## Project Structure

```
├── cmd/lazyomo/     # Entry point
├── internal/
│   ├── domain/         # Business models (Config, Group, Schema)
│   ├── application/    # Service layer (ConfigService)
│   ├── infrastructure/ # I/O (filesystem, backup)
│   ├── cli/            # CLI handler
│   └── tui/            # Bubble Tea TUI
│       └── components/ # TUI components
├── scripts/            # Build scripts
└── .github/workflows/  # CI/CD
```

## Code Guidelines

- Follow Go conventions (gofmt, go vet)
- Table-driven tests with `t.Run()`
- Domain layer: no I/O dependencies
- Infrastructure layer: no business logic
- TUI components: own styles struct to avoid circular imports

## Pull Requests

1. Fork the repo
2. Create a feature branch
3. Write tests for new functionality
4. Ensure `go test ./...` passes
5. Submit PR with clear description

## License

MIT
