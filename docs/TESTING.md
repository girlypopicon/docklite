# DockLite Testing Documentation

This document describes the automated testing infrastructure for DockLite.

## Overview

DockLite has comprehensive automated tests for both the Go backend and the TypeScript/Next.js frontend.

## Test Structure

```
docklite-new/
├── go-app/
│   └── internal/
│       ├── testhelpers/          # Test utilities and helpers
│       │   └── helpers.go        # Common test functions
│       ├── store/
│       │   ├── sqlite_test.go    # SQLite store tests
│       │   └── users_test.go     # User store tests
│       └── handlers/
│           ├── handlers_test.go  # Handler utility tests
│           └── ssl_test.go       # SSL domain validation tests
│
└── webapp/
    ├── vitest.config.ts          # Vitest configuration
    └── __tests__/
        ├── db.test.ts            # Database operation tests
        └── api.test.ts           # API integration tests
```

## Running Tests

### All Tests

```bash
make test
```

### Go Tests

```bash
# Run Go tests
make test-go

# Run Go tests with coverage
make test-go-coverage
```

Or directly:
```bash
cd go-app && go test -v -race ./...
```

### Webapp Tests

```bash
# Run webapp tests
make test-web

# Run webapp tests in watch mode
make test-web-watch
```

Or directly:
```bash
cd webapp && bun run test
```

## Test Categories

### Go Tests

#### Store Tests (`store/*_test.go`)
- Database creation and configuration
- User CRUD operations
- Password hashing and verification
- Concurrent access handling

#### SSL Tests (`handlers/ssl_test.go`)
- Domain validation regex
- Certificate parsing
- Expiry calculation
- Status determination

#### Handler Tests (`handlers/handlers_test.go`)
- Health endpoint
- JSON response helpers
- Uptime formatting
- Port formatting
- Template generation

### TypeScript Tests

#### Database Tests (`__tests__/db.test.ts`)
- User functions
- Site functions
- Database entity functions
- Folder functions
- Backup functions
- DNS functions

#### API Tests (`__tests__/api.test.ts`)
- Health endpoint
- Authentication
- Container endpoints
- User endpoints
- SSL endpoints
- Server stats

## Test Coverage

Generate coverage reports:

```bash
# All coverage
make test-coverage

# Go coverage only
make test-go-coverage
# Opens coverage.html in browser

# Webapp coverage only
make test-web-coverage
```

## Writing New Tests

### Go Tests

1. Create a new `*_test.go` file in the appropriate package
2. Use the test helpers from `internal/testhelpers`:

```go
package mypackage

import (
    "testing"
    "docklite-agent/internal/testhelpers"
)

func TestMyFunction(t *testing.T) {
    // Use in-memory database
    db := testhelpers.TestStoreWithTables(t)
    defer db.Close()

    // Your test code
    testhelpers.AssertEqual(t, expected, actual)
}
```

### TypeScript Tests

1. Create a new `*.test.ts` file in `webapp/__tests__/`
2. Use Vitest's describe/it pattern:

```typescript
import { describe, it, expect } from 'vitest'

describe('My Feature', () => {
    it('should do something', () => {
        expect(true).toBe(true)
    })
})
```

## CI/CD Integration

For CI/CD pipelines, use:

```bash
# Install dependencies
make install-test-deps

# Run all tests with coverage
make test-coverage

# Check exit code
echo $?  # 0 = all tests passed
```

## Test Best Practices

1. **Isolation**: Each test should be independent
2. **Cleanup**: Use `defer` (Go) or `afterEach` (TypeScript) for cleanup
3. **Descriptive names**: Test names should describe the expected behavior
4. **Coverage**: Aim for high coverage on critical paths
5. **Fast tests**: Use in-memory databases and mocks where possible
