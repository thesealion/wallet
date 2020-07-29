# Wallet

**Wallet** is a generic wallet service to use in various fintech startups.
It covers many account and payment-related needs such as transferring money
between accounts and listing accounts and payments.
It is written in Go with go-kit and it uses PostgreSQL to store data.

## Api documentation

Please see [api.md](/docs/api.md).

## Running

You can run the whole application along with the DB using Docker Compose:

```
docker-compose up
```

The DB is initially populated with three test accounts.

## Running tests

Start the database:

```
docker-compose -f docker-compose-test.yml up -d db
```

Run tests:

```
docker-compose -f docker-compose-test.yml up tests
```

Clean up:

```
docker-compose -f docker-compose-test.yml down
```
