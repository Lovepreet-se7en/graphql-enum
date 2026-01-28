# graphql-enum

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue.svg)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

GraphQL Path Enumeration Tool - Lists all possible paths from root queries/mutations to a target type in a GraphQL schema. Useful for security testing to identify potential authorization vulnerabilities across different access paths.

## Features

- **Dual Format Support**: Standard GraphQL introspection JSON and GitHub's custom schema format
- **Auto-Detection**: Automatically detects schema format
- **Path Enumeration**: Finds all routes from Query/Mutation to target types
- **Security Focused**: Identify authorization gaps where different paths have different permission checks
- **Colorized Output**: Easy-to-read terminal output
- **Fast & Efficient**: DFS with cycle detection and depth limiting

## Installation

### Via `go install` (Recommended)

```bash
go install github.com/lovepreet-se7en/graphql-enum@latest
```

This installs the `graphql-enum` binary to your `$GOPATH/bin` (make sure it's in your PATH).

### From Source

```bash
git clone https://github.com/lovepreet-se7en/graphql-enum.git
cd graphql-enum
go build -o graphql-enum
mv graphql-enum $GOPATH/bin/  # Or any directory in your PATH
```

## Usage

```bash
graphql-enum -schema schema.json -type User
```

### Options

- `-schema string`: Path to schema JSON file (required)
- `-type string`: Target type name to find paths to, case-sensitive (required)
- `-max-depth int`: Maximum traversal depth (default: 15)
- `-mutations`: Include Mutation fields as entry points (default: false)
- `-v`: Verbose output showing schema loading details
- `-no-color`: Disable colored output

### Examples

```bash
# Find all paths to Repository type
graphql-enum -schema github-schema.json -type Repository

# Include mutations as entry points
graphql-enum -schema schema.json -type Repository -mutations

# Increase depth for deeply nested schemas
graphql-enum -schema schema.json -type PullRequest -max-depth 20

# Verbose mode to see what's being loaded
graphql-enum -schema schema.json -type User -v

# Disable colors (useful for piping to other tools)
graphql-enum -schema schema.json -type Issue -no-color
```

## Schema Formats

### 1. Standard GraphQL Introspection

Result of the standard introspection query:

```json
{
  "data": {
    "__schema": {
      "queryType": { "name": "Query" },
      "types": [
        {
          "kind": "OBJECT",
          "name": "Query",
          "fields": [
            {
              "name": "user",
              "type": { "name": "User", "kind": "OBJECT" }
            }
          ]
        }
      ]
    }
  }
}
```

### 2. GitHub Custom Format

GitHub's schema documentation format:

```json
{
  "queries": [
    {
      "name": "repository",
      "type": "Repository",
      "args": [
        { "name": "owner", "type": "String!" },
        { "name": "name", "type": "String!" }
      ]
    }
  ],
  "objects": [
    {
      "name": "Repository",
      "fields": [
        { "name": "owner", "type": "User" }
      ]
    }
  ]
}
```

## Example Output

```
Target: Repository (OBJECT)
Entry points: Query
Max depth: 15

Found 8 paths:

  1. Query → viewer → repositories → Repository
  2. Query → repository(owner: String!, name: String!) → Repository
  3. Query → organization(login: String!) → repositories → Repository
  4. Query → user(login: String!) → repositories → Repository
  5. Query → search(query: String!, type: SearchType!) → Repository
  6. Query → enterprise(slug: String!) → organizations → repositories → Repository
  7. Query → node(id: ID!) → Repository
  8. Query → nodes(ids: [ID!]!) → Repository
```

## Security Use Case

GraphQL APIs can expose the same data through multiple paths. Developers may implement authorization checks inconsistently:

1. **Obtain schema**: Export via introspection or documentation
2. **Identify sensitive types**: User, PrivateRepository, PaymentInfo, etc.
3. **Enumerate paths**: Run `graphql-enum -schema schema.json -type SensitiveType`
4. **Test each path**: Verify all paths enforce proper authorization
5. **Report findings**: Document paths that bypass intended access controls

### Why This Matters

```graphql
# Path 1 - Has auth check
query { user(id: "123") { email } }

# Path 2 - Missing auth check (bypass!)
query { organization(id: "456") { members { email } } }
```

Both paths lead to `User.email`, but one might lack proper permission validation.

## Differences from Original

This is a Go rewrite of [graphql-path-enum](https://gitlab.com/dee-see/graphql-path-enum) with improvements:

| Feature | Original (Rust) | This Version (Go) |
|---------|----------------|-------------------|
| Schema formats | Introspection only | Introspection + GitHub custom |
| Auto-detection | ❌ | ✅ |
| Field arguments | ❌ | ✅ |
| Mutation handling | Basic | Smart return type detection |
| Type suggestions | ❌ | ✅ |
| Installation | Manual build | `go install` support |

## Dependencies

- [fatih/color](https://github.com/fatih/color) - Terminal colors

## License

MIT License - See [LICENSE](LICENSE) file.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Acknowledgments

- Original concept by [dee-see](https://gitlab.com/dee-see)
- Go rewrite with enhanced format support
