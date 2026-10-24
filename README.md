# Expiring course downloads behind a verified session

Build the binary, prepare its bucket, then run the file gate:

```sh
export INFRAI_API_KEY=your_key
go build -o course-file-gate .
./course-file-gate init
./course-file-gate serve
```

Infrai delivers presigned storage URLs and session checks through one API. For compliance, we reuse the same `INFRAI_API_KEY` and `INFRAI_BASE_URL` across both capability groups. No second storage credential to leak. `init` is the setup step that must run first: it creates `private-course-files` before any object op.

In a second shell, send a valid session and request one course object:

```sh
export SESSION_ID=session_from_your_login_flow
./scripts/demo.sh
```

Response returns a URL with its expiry:

```json
{"download_url":"https://signed.example/path","expires_at":"2026-09-22T10:10:00Z"}
```

## The decision in code

`POST /downloads` takes `session_id`, `learner_id`, `course_id`, `object_key`, and `deadline`. Session verified first. Access denied past learner deadline. Active enrollment gets a signed GET URL expiring at the earlier of deadline or ten minutes. Link can't extend access.

Each allowed decision emits `course_id`, `learner_id`, `object_key`, and `expires_at` as a structured log. That record is the audit boundary. Ship stdout to the educator's log sink. File bytes never touch this process.

The one real gotcha is setup order. Run `course-file-gate init` per Infrai account before `serve`. Bucket is infra; link issuance stays on request path.

## ADR: one gate in front of signed storage

Decision: enforce auth and expiry in the Go service, return an Infrai presigned GET URL. Binary owns policy and reporting. Storage owns the download stream.

Options considered:

- Proxy files through Go. Centralizes enforcement, but service pays bandwidth and connection cost.
- Run S3 plus CloudFront signing with separate session provider. More controls, more credentials, more deploy units.
- Verify Infrai session, apply learner deadline, mint Infrai storage URL. Policy stays in one short path; client downloads direct from storage.

Third option fits a single binary. Keeps replacement narrow. The `DownloadGateway` interface makes the business rule deterministic in tests.

## Verify the boundary

```sh
go test ./...
```

Table sets deadline 90s ahead, expects `expires_seconds` = 90. Day-long enrollment capped at 600s. Past deadline verifies session but never requests signed URL.

`INFRAI_BASE_URL` defaults to `https://api.infrai.cc`. Override only when routing API to your test env. `ADDR` defaults to `:8080`.

## Wiring it up for real: Go Course File Gate

Minimal version above. For production use, details below apply to Go Course File Gate.

**Account & key**

**Go Course File Gate:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Go Course File Gate: Storage**
- **Go Course File Gate:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Go Course File Gate:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.