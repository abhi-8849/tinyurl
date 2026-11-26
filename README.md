# tinyurl

A URL shortener written in Go. It turns a long URL into a 7-character code and redirects the code back to the long URL.

| Capability | Behavior |
| --- | --- |
| Shorten | A long URL becomes a unique 7-character Base62 code. |
| Redirect | `GET /{code}` answers `302` with the long URL in `Location`. |
| Expiry | Every link expires: at the time you set, or after the default lifetime (5 years). An expired link answers `410`. |
| Custom alias | An optional alias replaces the generated code. An alias that is already taken answers `409`. |
| Delete | Only the owner can delete. A deleted link answers `410`, and its code is never reused. Links are never updated. |

## High-level design

```
                          +-------------------------- tinyurl instance (run 1..N) -------------------------+
                          |                                                                                |
  create / delete  -----> |  Shorten service  --- id range ---+                                            |
  (X-API-Key)             |   validate, alias, id -> code     |                                            |
                          |                                   |                                            |
  GET /{code}      -----> |  Redirect service                 |                                            |
  (public)                |   cache first, then store         |                                            |
                          +---------+-------------------+-----+------------------------+-------------------+
                                    |                   |                              |
                              links (read/write)   cached links                    id counter
                                    |                   |                              |
                                    v                   v                              v
                           +-----------------+   +-------------+              +------------------+
                           | DynamoDB        |   | Redis       |              | DynamoDB         |
                           | url_mapping     |   | link:<code> |              | id_counter       |
                           +-----------------+   +-------------+              +------------------+
```

Every instance runs both services, so any instance can answer any request. Instances share state only through DynamoDB and Redis.

| Component | Job |
| --- | --- |
| Shorten service | Authenticates the caller, validates the URL and alias, picks a code, stores the link, and handles delete. |
| Redirect service | Looks up a code (cache first, then the store) and decides between `302`, `404` and `410`. |
| DynamoDB `url_mapping` | Source of truth for links. A conditional write keeps codes and aliases unique. Rows are never purged. |
| DynamoDB `id_counter` | One counter item. A server claims a range of ids with one atomic update, then issues ids from it locally. |
| Redis | Shared cache in front of `url_mapping`. Entries live for the time to expiry, at most 24 hours. |

Codes are made from ids: the id is multiplied by a fixed odd number modulo 62^7 (a one-to-one mapping, so codes never collide and do not look sequential),
then written in Base62 and padded to 7 characters.

## How the components interact

**Create with a generated code**
1. The caller sends `POST /api/v1/urls` with `X-API-Key`. A missing or unknown key answers `401`.
2. The Shorten service checks the URL: `http` or `https`, at most 2 KB, not pointing at this service. Otherwise `400`.
3. The generator hands out the next id from its current range. When 80% of the range is used, it claims the next range from the counter in the background.
4. The id becomes a 7-character code.
5. The link is written to `url_mapping` only if the code is unused.
6. The link is written to Redis, so the first redirect is a cache hit. A Redis error is logged and ignored.
7. The response is `201` with the code, short URL and expiry.

**Create with a custom alias**
1. Steps 1 and 2 are the same as above.
2. The alias must be 4 to 30 characters from `A-Za-z0-9_-`, not `api` or `metrics`, and not exactly 7 letters or digits (that shape belongs to generated codes). Otherwise `400`.
3. The generator is skipped. The alias is written to `url_mapping` only if it is unused; if it is taken, the answer is `409`.
4. Steps 6 and 7 are the same as above.

**Redirect**
1. `GET /{code}` is public.
2. The Redirect service reads Redis. A hit goes to step 5.
3. On a miss, it reads `url_mapping` (eventually consistent). Concurrent misses for one code share a single read. A code with no row answers `404`.
4. The row is written to Redis for the time to expiry, at most 24 hours.
5. If the link is deleted, the answer is `410`. If its expiry has passed, the answer is `410`.
6. Otherwise the answer is `302` with `Location` set to the long URL. A Redis error at any point is logged and treated as a miss.

**Delete**
1. The caller sends `DELETE /api/v1/urls/{code}` with `X-API-Key`. A missing or unknown key answers `401`.
2. An unknown code answers `404`. A link owned by someone else answers `403`.
3. The row gets a `deleted_at` time. It is not removed, so the code can never be issued again.
4. The Redis entry is removed, and the answer is `204`. Deleting an already deleted link also answers `204`.

## Failure behavior

| Failure | What happens |
| --- | --- |
| Redis down | Redirects are served from DynamoDB, a little slower. Creates and deletes still work. Errors are logged and counted in `tinyurl_cache_errors_total`. |
| Counter item unavailable | Servers keep issuing codes from the ranges they already hold. Redirects are not affected. Creates fail only when a server has used up its range. |
| DynamoDB down | Links already in Redis still redirect. Creates, deletes and redirects for uncached links fail. |
| Instance crash | The unused part of its id range is lost. This is harmless: ids are plentiful and codes only need to be unique. |

## API

Create a link. `custom_alias` and `expires_at` (RFC 3339) are optional.

```
curl -s -H 'X-API-Key: k1' -X POST localhost:8080/api/v1/urls \
  -d '{"long_url":"https://example.com","custom_alias":"my-link","expires_at":"2030-01-01T00:00:00Z"}'
# 201 {"code":"my-link","short_url":"http://localhost:8080/my-link","expires_at":"2030-01-01T00:00:00Z"}
```

Follow a link:

```
curl -I localhost:8080/my-link
# 302 with Location: https://example.com
```

Delete a link:

```
curl -i -H 'X-API-Key: k1' -X DELETE localhost:8080/api/v1/urls/my-link
# 204
```

| Request | Statuses |
| --- | --- |
| `POST /api/v1/urls` | `201`, `400` bad URL, alias or body, `401`, `409` alias taken |
| `GET /{code}` | `302`, `404` unknown, `410` expired or deleted |
| `DELETE /api/v1/urls/{code}` | `204`, `401`, `403` not the owner, `404` |
| `GET /metrics` | Prometheus metrics |

## Run it

Run locally with no infrastructure: links are kept in memory and ids come from a local counter.

```
TINYURL_API_KEYS=k1:alice,k2:bob go run ./cmd/tinyurl
```

### Configuration

| Flag | Default | Meaning |
| --- | --- | --- |
| `-addr` | `:8080` | Listen address. |
| `-base-url` | `http://localhost:8080` | Public address that short URLs start with. |
| `-store` | `memory` | `memory` or `dynamodb`. |
| `-dynamodb-endpoint` | empty | Set to use DynamoDB Local. Empty uses AWS. |
| `-table` | `url_mapping` | DynamoDB table for links. |
| `-counter-table` | `id_counter` | DynamoDB table for the id counter. |
| `-create-tables` | `false` | Create the DynamoDB tables if they are missing. |
| `-redis-addr` | empty | Redis address. Empty uses an in-memory cache. |
| `-range-size` | `1000000` | Ids claimed from the counter at a time. |
| `-default-ttl` | `43800h` | Lifetime of a link with no expiry of its own (5 years). |

`TINYURL_API_KEYS` holds the API keys as `key:owner` pairs separated by commas. The server does not start without it.
With DynamoDB, set `AWS_REGION`, `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`; DynamoDB Local accepts any values.

Logs are JSON on stderr: one line per request, plus range refills and cache errors. API keys are never logged.
Metrics at `/metrics`: `tinyurl_redirects_total{result}`, `tinyurl_redirect_duration_seconds`, `tinyurl_creates_total{kind}`,
`tinyurl_alias_conflicts_total`, `tinyurl_deletes_total`, `tinyurl_cache_errors_total`, `tinyurl_range_refills_total`, `tinyurl_range_refill_errors_total`.

### Docker Compose

Starts DynamoDB Local, Redis and two tinyurl instances on ports 8081 and 8082.

```
docker compose -f deploy/docker-compose.yml up --build
curl -s -H 'X-API-Key: k1' -X POST localhost:8081/api/v1/urls -d '{"long_url":"https://example.com"}'
curl -I localhost:8082/<code>
```

The two instances claim different id ranges, so their codes never overlap, and a link created on one redirects from the other.

### Kubernetes

For a local cluster such as kind. This starts DynamoDB Local, Redis and two tinyurl pods.

```
docker build -t tinyurl:latest -f deploy/Dockerfile .
kind load docker-image tinyurl:latest
kubectl apply -f deploy/k8s
kubectl get pods
kubectl port-forward svc/tinyurl 8080:80
```

Then use the API examples above against `localhost:8080`. The pods use TCP probes on port 8080, because the server has no health endpoint.
The API keys are in the `tinyurl-keys` Secret in `deploy/k8s/tinyurl.yaml`; change them for anything beyond local use.
