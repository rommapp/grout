# Contributing to Grout

First off, thank you for considering contributing to Grout! We appreciate your time and effort.

Before implementing major features, please open an issue or discuss it in the
[Grout Development Channel](https://discord.com/channels/1138838206532554853/1456747141518069906) on the
[RomM Discord](https://discord.gg/P5HtHnhUDH) to ensure alignment with the project's direction.

## Code of Conduct

By contributing, you agree to uphold our [Code of Conduct](CODE_OF_CONDUCT.md). Be respectful, constructive, and
welcoming to all contributors regardless of experience level.

## Getting Started

1. Read the [Development Guide](DEVELOPMENT.md) for setup instructions
2. Fork and clone the repository
3. Create a feature branch from `main` with a descriptive name (e.g., `feature/add-search-filter`)
4. Make your changes
5. Test locally on your target CFW if possible
6. Commit with clear, descriptive messages
7. Push and open a pull request against `main`

## Pull Request Standards

- Follow existing code conventions and patterns
- Test your changes locally before submitting
- Update documentation if your changes affect user-facing behavior
- Ensure the build passes (`task build` or `go build ./...`)
- Provide a clear description of what your PR does and why

## Code Style

We use standard Go formatting and conventions:

- Run `go fmt` before committing
- Use `go vet` and `staticcheck` to catch issues
- Follow existing patterns in the codebase for consistency

## Reporting Bugs

Please [create an issue](https://github.com/rommapp/grout/issues/new/choose) with:

- Your CFW and version (muOS, Knulli, Spruce, NextUI)
- Grout version
- Steps to reproduce the issue
- Expected vs actual behavior
- Any relevant logs or screenshots

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (GPL-3.0).
