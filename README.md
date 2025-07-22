# Proof Of Work for Word of Wisdom

## Task

Test task for Server Engineer

Design and implement “Word of Wisdom” TCP server.

- TCP server should be protected from DDOS attacks with the [Proof of Work](https://en.wikipedia.org/wiki/Proof_of_work), the challenge-response protocol should be used.
- The choice of the POW algorithm should be explained.
- After Proof Of Work verification, server should send one of the quotes from “word of wisdom” book or any other collection of the quotes.
- Docker file should be provided both for the server and for the client that solves the POW challenge.
- No restrictions on tools usage.

---

## Usage

### Method #1 (Makefile)
```sh
make run_client
make run_server
```

### Method #2 (docker-compose)
```sh
docker-compose up -d

docker logs pow-wow_client_1 -f
# or
docker logs pow-wow_server_1 -f
```

---

## Environment Variables

### Client
| name           | type    | default        | description
|----------------|---------|----------------|--------------------------------------
| SERVER_ADDR    | string  | 127.0.0.1:9000 | TCP server address
| FETCH_WORKERS  | int     | 4              | number of concurrent client requests
| TIMEOUT        | int     | 1000           | timeout after failed request (ms)

### Server
| name             | type    | default        | description
|------------------|---------|----------------|----------------------------------------
| LISTEN_ADDR      | string  | 0.0.0.0:9000   | server TCP address
| DIFFICULTY       | byte    | 23             | PoW difficulty (leading zero bits)
| PROOF_TOKEN_SIZE | int     | 64             | salt size for PoW (bytes)

---

## Implementation Overview

### Project Structure
- `deploy/` – Dockerfiles, nginx config
- `cmd/` – entrypoints for server and client
- `internal/pow/` – Argon2id PoW implementation
- `internal/client/` – client logic
- `internal/server/` – server logic

### Stateless Argon2id Challenge-Response Protocol
- **Client** fetches a challenge from the HTTP/3 endpoint `/challenge` (QUIC, UDP, TLS; fallback to HTTP/1.1/2 if needed).
- **Challenge** is a JSON object: `{ difficulty, salt, signature }`, where signature is an HMAC for stateless validation.
- **Client** solves the PoW: finds a nonce such that `Argon2id(salt|nonce)` has the required number of leading zeros.
- **Client** connects to the TCP server and sends `{ difficulty, salt, signature, nonce }` as JSON.
- **Server** validates the signature and PoW solution statelessly.
- If valid: server sends a random quote and closes the connection.
- If invalid: server closes the connection.
- **Nginx** supported and `/challenge` can be used to offload solution check without touching entire service

### Security & DDoS Protection
- **Memory-hard PoW** (Argon2id) is resistant to botnets and ASICs.
- **Stateless**: no server-side challenge tracking required.
- **nginx** (or direct Go HTTP/3) can rate-limit challenge requests.
- **Connection limiting** and timeouts on the TCP server.
- **Healthchecks**: `/healthz` endpoint checks both HTTP and TCP server status.
- **Graceful shutdown**: both HTTP and TCP servers shut down cleanly on SIGINT/SIGTERM.

### Deployment
- The server Docker image exposes TCP (9000) and HTTP/3 (8443, QUIC+TLS) ports.
- nginx can be used for HTTP/1.1/2 fallback and rate limiting, but HTTP/3 is served directly by Go for best performance.

### Argon2id PoW Algorithm
- **Why Argon2id?**
  - Memory-hard, modern, and secure.
  - Resistant to specialized hardware attacks.
- **Parameters** (default):
  - Time: 2
  - Memory: 64 MB
  - Parallelism: 2
  - Key length: 32 bytes
- **Calculation:**
  - Find a nonce such that `Argon2id(salt|nonce)` has at least `difficulty` leading zero bits.

---

## Possible Upgrades & Improvements

- **Dynamic Difficulty**: Adjust PoW difficulty based on server load or attack detection.
- **Per-IP Rate Limiting**: At the HTTP/3 and TCP layers.
- **Blacklist/Throttle**: Block or slow abusive IPs.
- **Distributed Challenge Service**: Use CDN or global edge to distribute challenges.
- **Optional**: Use a memory-hard PoW variant with tunable parameters for future-proofing.
- **Fallback**: Support HTTP/1.1/2 for legacy clients, but prefer HTTP/3

---

## References
- [Argon2id PoW](https://en.wikipedia.org/wiki/Argon2)
- [QUIC/HTTP3](https://datatracker.ietf.org/doc/html/rfc9000)
- [Proof of Work](https://en.wikipedia.org/wiki/Proof_of_work)
