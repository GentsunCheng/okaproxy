# Contributing to OkaProxy

Thank you for your interest in contributing to OkaProxy! We welcome contributions from the community and are grateful for your help in making this project better.

## 🚀 Getting Started

### Prerequisites

Before you begin, ensure you have the following installed:
- Go 1.23 or later
- Git
- Make (optional but recommended)
- Redis (for local development)
- Docker and Docker Compose (optional)

### Setting Up Development Environment

1. **Fork and Clone the Repository**
   ```bash
   git clone https://github.com/GentsunCheng/okaproxy.git
   cd okaproxy
   ```

2. **Install Development Tools**
   ```bash
   make setup
   ```

3. **Initialize Configuration**
   ```bash
   make init-config
   # Edit config.toml with appropriate settings for development
   ```

4. **Start Redis (if not using Docker)**
   ```bash
   redis-server
   ```

5. **Run the Application**
   ```bash
   make run
   ```

## 📝 Development Workflow

### 1. Create a Feature Branch
```bash
git checkout -b feature/your-feature-name
```

### 2. Make Your Changes
- Write clean, well-documented code
- Follow Go best practices and conventions
- Add tests for new functionality
- Update documentation as needed

### 3. Test Your Changes
```bash
# Run all tests
make test

# Run linting and formatting
make fmt
make vet
make lint

# Run security checks
make security

# Run all CI checks
make ci
```

### 4. Commit Your Changes
```bash
git add .
git commit -m "feat: add new feature description"
```

### 5. Push and Create Pull Request
```bash
git push origin feature/your-feature-name
```

Then create a Pull Request on GitHub with a clear description of your changes.

## 🎯 Types of Contributions

### 🐛 Bug Reports
When reporting bugs, please include:
- Go version and OS
- OkaProxy version
- Configuration file (with sensitive data removed)
- Steps to reproduce
- Expected vs actual behavior
- Log output (if relevant)

### 💡 Feature Requests
When suggesting features:
- Describe the use case
- Explain why it would be valuable
- Consider backward compatibility
- Provide implementation ideas if possible

### 📚 Documentation
- Fix typos or improve clarity
- Add examples and use cases
- Translate documentation
- Improve API documentation

### 🔧 Code Contributions
- Bug fixes
- New features
- Performance improvements
- Security enhancements
- Test coverage improvements

## 📋 Code Guidelines

### Go Conventions
- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` and `goimports` for formatting
- Write clear, descriptive variable and function names
- Add comments for exported functions and complex logic
- Keep functions focused and small

### Project Structure
```
okaproxy/
├── main.go                 # Application entry point
├── internal/               # Internal packages (not importable)
│   ├── config/            # Configuration management
│   ├── logger/            # Logging functionality
│   ├── middleware/        # HTTP middleware
│   ├── proxy/             # Proxy logic
│   └── server/            # Server management
├── public/                # Static files
├── docs/                  # Documentation
└── tests/                 # Integration tests
```

### Naming Conventions
- Files: `snake_case.go`
- Packages: `lowercase`
- Functions/Methods: `PascalCase` (exported) or `camelCase` (internal)
- Constants: `PascalCase` or `SCREAMING_SNAKE_CASE`
- Variables: `camelCase`

### Error Handling
- Use the standard Go error handling pattern
- Wrap errors with context using `fmt.Errorf`
- Log errors at appropriate levels
- Return meaningful error messages

### Testing
- Write unit tests for all new functionality
- Use table-driven tests where appropriate
- Mock external dependencies
- Aim for >80% test coverage
- Include integration tests for critical paths

### Commit Message Format
Follow [Conventional Commits](https://conventionalcommits.org/):

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only changes
- `style`: Formatting, missing semi colons, etc.
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `perf`: Performance improvement
- `test`: Adding missing tests
- `chore`: Changes to the build process or auxiliary tools

Examples:
```
feat(auth): add JWT token validation
fix(proxy): resolve connection leak issue
docs: update installation guide
test(middleware): add rate limiting tests
```

## 🔍 Code Review Process

### For Contributors
- Ensure your PR passes all CI checks
- Keep PRs focused and atomic
- Write clear PR descriptions
- Respond to review comments promptly
- Update documentation if needed

### For Reviewers
- Be constructive and respectful
- Focus on code quality and maintainability
- Check for security implications
- Verify tests cover the changes
- Ensure documentation is updated

## 🚦 Pull Request Guidelines

### Before Submitting
- [ ] Code follows project conventions
- [ ] All tests pass (`make test`)
- [ ] Code is properly formatted (`make fmt`)
- [ ] No linting errors (`make lint`)
- [ ] Security checks pass (`make security`)
- [ ] Documentation is updated
- [ ] Commit messages follow conventions

### PR Description Template
```markdown
## Description
Brief description of the changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing completed

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Comments added for complex code
- [ ] Documentation updated
- [ ] No breaking changes (or documented)
```

## 🏷️ Release Process

### Versioning
We use [Semantic Versioning](https://semver.org/):
- MAJOR: Incompatible API changes
- MINOR: Backward-compatible functionality
- PATCH: Backward-compatible bug fixes

### Release Steps
1. Update version in relevant files
2. Update CHANGELOG.md
3. Create and push version tag
4. GitHub Actions will build and publish release

## 🤝 Community Guidelines

### Code of Conduct
- Be respectful and inclusive
- Welcome newcomers and help them learn
- Focus on constructive criticism
- Respect different viewpoints and experiences

### Communication
- Use English for all communications
- Be clear and concise
- Provide context and examples
- Ask questions if something is unclear

### Getting Help
- Check existing issues and documentation first
- Use GitHub Discussions for questions
- Join our community chat (if available)
- Tag maintainers only when necessary

## 🎖️ Recognition

Contributors will be recognized in:
- README.md contributors section
- Release notes
- Project documentation
- Community showcases

## 📞 Contact

- GitHub Issues: [Bug reports and feature requests](https://github.com/GentsunCheng/okaproxy/issues)
- GitHub Discussions: [General questions and discussions](https://github.com/GentsunCheng/okaproxy/discussions)
- Email: [maintainer email if available]

## 📚 Additional Resources

- [Go Documentation](https://golang.org/doc/)
- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Redis Documentation](https://redis.io/documentation)
- [Gin Framework Guide](https://gin-gonic.com/docs/)

Thank you for contributing to OkaProxy! 🚀