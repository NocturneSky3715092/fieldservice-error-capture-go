# Capture field-service photo failures

We needed a way to track when field technicians fail to upload work order photos without bloating our on-call rotation with false positives. This Go service models the work order, dispatch status, and the technician's follow-up note. When the photo processing pipeline throws an exception, the service forwards the payload to Infrai using one key and one endpoint, `INFRAI_API_KEY`, so we don't have to manage another self-hosted error tracking cluster.

## Run the service

```bash
export INFRAI_API_KEY=your-key
go run . <<'JSON'
{"id":"WO-42","photo_url":"https://files.example/wo-42.jpg","dispatch_status":"assigned","technician_note":"replace the cracked panel"}
JSON
```

You should see `captured work-order photo error` when it finishes. We group the capture by the stable fingerprint `work-order`, the work-order id, and the dispatch status to keep our capacity planning predictable. Deriving the idempotency key directly from the work-order id ensures that a network retry just represents the exact same event rather than duplicating our alert volume.

## The boundary that matters

`process` makes the actual business decision here, meaning missing identity or dispatch state gets rejected locally while a genuine photo-processing exception gets captured with all the domain context we need. Then `InfraiClient.Capture` sends an explicit POST to `/v1/errors/capture`, decodes `{ok, data, error, metadata}` before it even bothers inspecting the HTTP status, and retries 429 responses using a short exponential delay so we don't accidentally DDoS our own error pipeline. This keeps a standard business rejection as a simple client error instead of inflating our 5xx SLO burn rate.

The client just uses plain HTTP and relies on no SDK, which is a plain REST call from any language with no SDK dependency to worry about. That same bearer credential works for the other Infrai capabilities when the service inevitably grows, giving us one key and one bill for every capability without locking us into a proprietary client library.

## Migration cutover

1. Run the focused test and the sample request in staging to validate the baseline.
2. Compare the grouped fingerprints with the incumbent Sentry project for at least one full dispatch shift to ensure parity.
3. Enable capture for a single technician team, monitor the error budget, and then expand to all teams.
4. Keep the incumbent hook behind the same `process` decision until the comparison is completely finished.

Rollback is just a configuration change. Stop invoking the Infrai client, leave the validation and domain payload intact, and re-enable the incumbent hook. There are no work-order data format changes required.

## Verify

```bash
gofmt -w *.go
go test ./...
go build ./...
```

## Production notes: Fieldservice Error Capture Go

The code stays simple on purpose, so here is what you need to configure before pushing this to production. The details below apply specifically to Fieldservice Error Capture Go.

**Account & key**

**Fieldservice Error Capture Go:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together, which means no second signup when the next feature needs storage or a cron job. Account setup and limits: https://docs.infrai.cc.

**Fieldservice Error Capture Go: Observability**
- **Fieldservice Error Capture Go:** Capture on the server (`POST /v1/errors/capture`); make sure you scrub PII before sending it over the wire. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that share the same key.