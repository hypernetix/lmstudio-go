# Contribution Guide

Thank you for contributing to this project! To maintain consistency and clarity in our commit history, we follow a structured commit message format based on common Git patch notations.

## Commit Message Format

Each commit message should follow this format:

```plaintext
<type>(<module>): <description>
```

- **`<type>`**: The type of change (see the list below).
- **`<module>` (optional)**: The specific part of the project affected.
- **`<description>`**: A concise summary of the change in the imperative mood (e.g., "Fix bug in auth module" instead of "Fixed bug").

## Accepted Commit Types

| Type         | Meaning |
|--------------|---------|
| **feat**     | A new feature |
| **fix**      | A bug fix |
| **tech**     | A technical improvement or change |
| **cleanup**  | Code cleanup |
| **refactor** | Code restructuring without functional changes |
| **test**     | Adding or modifying tests |
| **docs**     | Documentation updates |
| **style**    | Code style changes (whitespace, formatting, etc.) |
| **chore**    | Miscellaneous tasks (e.g., dependency updates, tooling changes) |
| **perf**     | Performance improvements |
| **ci**       | Changes to CI/CD configurations |
| **build**    | Changes to the build system or dependencies |
| **revert**   | Reverting a previous commit |
| **security** | Security fixes |

## Examples

### Feature Addition
```plaintext
feat(auth): add OAuth2 support for login
```
### Bug Fix
```plaintext
fix(ui): resolve button alignment issue on mobile
```

### Technical Improvement
```plaintext
tech(database): add errorx abstraction for go-lang, database and API errors
```

### Code Refactoring
```plaintext
refactor(database): optimize query execution
```
### Test Addition
```plaintext
test(api): add unit tests for user authentication
```
### Documentation Update
```plaintext
docs(readme): update installation instructions
```
### Style Change
```plaintext
style(css): apply consistent spacing in stylesheet
```

## Best Practices
- Keep commit messages **concise** (ideally under 50 characters for the title).
- Use **imperative mood** (e.g., "Fix bug" instead of "Fixed bug").
- Ensure commits are **atomic** (one logical change per commit).
- If needed, provide additional details in the commit body.
- Follow existing coding and commit standards.

## Pre-commit testing
- Keep libs/* code coverage above 80% (run `make coverage` to check)
- Do not forget to add tests to tests/* (run `tests/api_tests.py` to check)

---

By following this guide, we can maintain a clean and structured commit history that is easy to read and understand. Happy coding!
