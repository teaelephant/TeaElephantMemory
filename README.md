# Tea Elephant Memory

Tea Elephant Memory is a service for managing tea collections, providing information about tea, and helping users track their tea inventory.

## Installation

### Prerequisites

- Go 1.23 or higher
- PostgreSQL 13 or higher (if using PostgreSQL as the database)
- FoundationDB (if using FoundationDB as the database)

### Using Docker Compose

The easiest way to run Tea Elephant Memory is using Docker Compose:

```bash
docker-compose up -d
```

This will start the server and the required database (PostgreSQL by default).

## Configuration

Tea Elephant Memory can be configured using environment variables:

- `LOG_LEVEL`: Log level (default: "info")
- `DATABASE_TYPE`: Database type to use, either "fdb" or "postgres" (default: "fdb")
- `POSTGRES_CONNECTION_STRING`: PostgreSQL connection string (required if DATABASE_TYPE is "postgres")
- `DATABASEPATH`: Path to FoundationDB cluster file (default: "/usr/local/etc/foundationdb/fdb.cluster", only used if DATABASE_TYPE is "fdb")
- `OPEN_AI_TOKEN`: OpenAI API token (required)

## Database Support

Tea Elephant Memory supports two database backends:

### PostgreSQL

To use PostgreSQL as the database backend, set the following environment variables:

```
DATABASE_TYPE=postgres
POSTGRES_CONNECTION_STRING=postgres://username:password@hostname:port/database?sslmode=disable
```

### FoundationDB

To use FoundationDB as the database backend, set the following environment variables:

```
DATABASE_TYPE=fdb
DATABASEPATH=/path/to/fdb.cluster
```
