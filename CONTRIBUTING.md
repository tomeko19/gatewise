# Contributing to Gatewise

Thank you for your interest in contributing to Gatewise! 🎉

## How to Contribute

### Reporting Bugs
- Use GitHub Issues
- Include clear description, steps to reproduce, and expected vs actual behavior
- Add relevant logs and screenshots

### Suggesting Features
- Open a GitHub Discussion first to discuss the feature
- Explain the use case and benefits
- Consider if it fits the Community or Enterprise tier

### Code Contributions

1. **Fork the repository**
2. **Create a feature branch**: `git checkout -b feature/my-feature`
3. **Make your changes**:
   - Follow existing code style
   - Add tests for new features
   - Update documentation
4. **Test your changes**:
   ```bash
   # Backend tests
   cd backend && pytest
   
   # Frontend tests
   cd frontend && yarn test
   
   # Go tests
   cd gatewise && go test ./...
   ```
5. **Commit with clear messages**: `git commit -m "feat: add amazing feature"`
6. **Push to your fork**: `git push origin feature/my-feature`
7. **Open a Pull Request**

### Code Style

- **Go**: Follow standard Go conventions (`gofmt`, `golint`)
- **Python**: PEP 8 style guide
- **JavaScript/React**: ESLint with Airbnb style

### Commit Message Format

Use conventional commits:
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `refactor:` - Code refactoring
- `test:` - Adding tests
- `chore:` - Maintenance tasks

## Development Setup

See [README.md](README.md) for detailed setup instructions.

## Questions?

Open a GitHub Discussion or contact us at support@gatewise.io

Thank you for contributing! 🚀
