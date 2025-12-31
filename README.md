# Project Overview

This project is a web service for managing kids' check-ins and check-outs, likely for a church or similar organization, given the references to "Planning Center". It's built in Go and uses the Fiber web framework. The service exposes a RESTful API and also uses websockets for real-time updates. The data is stored in a SQLite database.

The project is structured as a command-line application with two main commands:
- `apiserver`: Starts the web server.
- `checkout-fetcher`: A background worker that fetches checkout information from Planning Center.

## Requirements
- [Golang](https://go.dev/) 1.25+
- [SQLite](https://www.sqlite.org/) (should be on macOS by default)
- [GNU make](https://www.gnu.org/software/make/) (should be on macOS by default)

## Quick Start
1. Initialize the database:
```shell
make db-reset
```
2. In one terminal, start the checkout fetcher:
```shell
make checkout-fetcher
```
3. In another terminal, start the server:
```shell
make web
```
4. Navigate to http://localhost:3000/v1/checkins/checkouts/Test%20Location?checked_out_after=-31m to see the checkouts for the 31 past minutes.

## Building and Running

The project uses a `Makefile` for common tasks. Run `make` to see a list of available targets.

### Building the application

To build the application binary, run:

```sh
make build
```

This will create a binary at `./bin/kids-checkin`. Running `./bin/kids-checkin` will print the help message.

### Running the web server

To run the web server, use the `web` target:

```sh
make web
```

This will build the application and start the API server on port `3000` by default.

### Running the checkout fetcher

To run the checkout fetcher, use the `checkout-fetcher` target:

```sh
make checkout-fetcher
```

This will build the application and start the fetcher process.

### Running tests

To run the test suite, use the `test` target:

```sh
make test
```

## Database

The project uses SQLite for its database. Database migrations are managed with the `migrate` tool.

- **Resetting the database:** `make db-reset`
- **Running migrations:** `make db-migrate`
- **Creating a new migration:** `make db-new-migration NAME=<migration_name>`

## API Endpoints

The API is currently versioned under `/v1`.

### Check-ins

- `GET /v1/checkins/checkouts/:location`: Get a list of checkouts for a specific location. This endpoint can also be upgraded to a websocket connection for real-time updates.

### Locations

- `GET /v1/locations`: Get a list of locations.

## Development Conventions

- The project follows standard Go project layout.
- It uses `gofiber` for the web framework.
- The `urfave/cli` library is used for the command-line interface.
- Database queries are built using `squirrel`.
- Testing is done with the standard `testify` library.
