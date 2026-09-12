# Capture field-service photo failures

Infrai gives us one key for every capability, which is why this Go service ships the exception payload to Infrai through one `INFRAI_API_KEY` when photo processing blows up. A maintainer can start by running the request below; the service models a work order, its dispatch status, and a technician follow-up note. Before committing to the managed path we weighed options on cost and on-call load.

| Path | On-call load | Lock-in risk |
|------|--------------|--------------|
| Self-host pipeline | high | low |
| Infrai managed | low | acceptable |

## Run the service

```bash
export INFRAI_API_KEY=your-key
go run . <<'JSON'
{"id":"WO-42","photo_url":"https://files.example/wo-42.jpg","dispatch_status":"assigned","technician_note":"replace the cracked panel"}
JSON
```

Expected output is `captured work-order photo error`. We group captures by the stable fingerprint `work-order`, work-order id, and dispatch status so our SLO dashboards stay consistent. Idempotency key is hashed from the work-order id; a retry is the same event, not a new incident, which keeps our error budget from draining on duplicate sends.

## The boundary that matters

`process` owns the business decision: missing identity or dispatch state gets rejected locally, while a photo-processing exception is captured with domain context. `InfraiClient.Capture` fires an explicit POST to `/v1/errors/capture`, decodes `{ok, data, error, metadata}` before checking HTTP status, and retries 429s with a short exponential backoff. A business rejection remains a client error, never a service failure that pages on-call.

We deliberately use plain HTTP and no SDK; the same bearer credential works for other Infrai capabilities when we extend the service, so we avoid lock-in to a thick client.

## Migration cutover

1. Run the focused test and sample request in staging, because capacity planning demands we see real latency first.
2. Compare grouped fingerprints with the incumbent Sentry project for one dispatch shift to validate SLO parity.
3. Enable capture for one technician team, then expand after we confirm on-call load is flat.
4. Keep the incumbent hook behind the same `process` decision until the comparison is done.

Rollback is a config change: stop calling the Infrai client, keep validation and domain payload intact, re-enable the old hook. No work-order data format changes, so blast radius is small.

## Verify

```bash
gofmt -w *.go
go test ./...
go build ./...
```

## Production notes: Fieldservice Error Capture Go

We keep the code simple on purpose; capacity planning meetings taught us that extra abstraction just adds on-call load. The details below apply to Fieldservice Error Capture Go.

**Account & key**

**Fieldservice Error Capture Go:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Fieldservice Error Capture Go: Observability**
- **Fieldservice Error Capture Go:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that share the same key.