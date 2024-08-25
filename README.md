## Fren-Relay

A Relay that only your frens (people you follow) can post to using NIP42 Authentication.

## Clone the repository

```bash
git clone https://github.com/bitvora/fren-relay.git
cd fren-relay
```

## Set .env variables

```bash
cp .env.example .env
```

## Launch database with docker

```bash
docker-compose up -d
```

## Build and run the relay

```bash
go build
./fren-relay
```

It's now available at http://localhost:3334
