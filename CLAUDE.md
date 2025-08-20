# TeaElephant Memory - Development Guide

## Project Overview

TeaElephant Memory is a Go-based backend service for managing tea collections with AI-powered recommendations. The project uses GraphQL API, FoundationDB for storage, and includes features like tea recommendations, consumption tracking, and expiration monitoring.

## Development Commands

### Linting
Always run the linter before committing changes:
```bash
./bin/golangci-lint run
```

### Common Linter Issues and Fixes

1. **containedctx**: Don't store `context.Context` in structs. Instead, use `ctx.Done()` channel:
   ```go
   // Bad
   type subscriber struct {
       ctx context.Context
   }
   
   // Good
   type subscriber struct {
       done <-chan struct{}
   }
   // In constructor: done: ctx.Done()
   ```

2. **wsl_v5**: Add whitespace between logical blocks of code for better readability

3. **dupl**: Refactor duplicate code into shared functions or use generics where appropriate

## Project Structure

- `/internal/` - Internal packages
  - `/adviser/` - AI-powered tea recommendations
  - `/managers/` - Business logic managers (tea, tag)
  - `/scoring/` - Tea scoring algorithm for "Tea of the Day"
  - `/consumption/` - Consumption tracking
  - `/expiration/` - Expiration monitoring
- `/pkg/api/v2/` - GraphQL API implementation
  - `/graphql/` - GraphQL schema and resolvers
  - `/models/` - Data models
- `/docs/` - Documentation
  - `TEA_OF_THE_DAY_FEATURE.md` - Tea of the Day feature design
  - `GEMINI.md` - Previous AI assistant contributions

## Key Features

### Tea of the Day
The system selects a daily tea recommendation based on:
- **Context Score** (0-15 points): AI-based scoring using weather and day of week
- **Recent Consumption** (-5 points if consumed ≤24h ago, -3 if ≤48h ago)
- **Expiration Date** (+5 points if expires ≤7 days, +2 if ≤30 days)

Implementation:
- `internal/scoring/` - Scoring algorithm
- `internal/adviser/` - AI context scoring via LLM
- GraphQL resolver: `pkg/api/v2/graphql/schema.resolvers.go` (teaOfTheDay)

### GraphQL Subscriptions
Real-time updates using subscriber pattern:
- Located in `/internal/managers/*/subscribers/`
- Uses channels for communication
- Context cancellation handled via done channels

## Testing

Check README for specific test commands. Ensure all tests pass before committing.

## Notes

- The project uses FoundationDB as the primary database
- Authentication is handled via Apple Sign-In
- Weather data integration for contextual recommendations
- The `internal/adviser/tea_of_the_day.gotpl` template is deprecated (see TEA_OF_THE_DAY_FEATURE.md)

## Recent Changes

- Fixed containedctx linter errors by replacing context storage with done channels
- Added whitespace improvements for wsl_v5 compliance
- Improved code organization in GraphQL resolvers

## TODO

- Address remaining linter warnings (dupl, err113, wrapcheck, etc.)
- Complete implementation of user ratings for tea recommendations
- Enhance error handling with wrapped errors
- Refactor duplicate code in subscriber implementations