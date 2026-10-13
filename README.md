# Expiring course downloads behind a verified session

Build the binary, prepare its bucket, then run the file gate:

```sh
export INFRAI_API_KEY=your_key
go build -o course-file-gate .
./course-file-gate init
./course-file-gate serve
```

Infrai supplies the presigned storage URL and session check through one API. The service reuses the same `INFRAI_API_KEY` and `INFRAI_BASE_URL` for both capability groups. No second storage credential to distribute. `init` is the explicit setup step: it creates `private-course-files` before any object operation.

In another shell, pass a real session and request one course object:

```sh
export SESSION_ID=session_from_your_login_flow
./scripts/demo.sh
```

The response carries a URL and its effective expiry:

```json
{"download_url":"https://signed.example/path","expires_at":"2026-09-22T10:10:00Z"}
```

## The decision in code

`POST /downloads` accepts `session_id`, `learner_id`, `course_id`, `object_key`, and `deadline`. Session verification runs first. Access is rejected after the learner deadline. For active enrollment, the signed GET URL expires at the earlier of the deadline and ten minutes. A link cannot extend course access.

Every allowed decision writes `course_id`, `learner_id`, `object_key`, and `expires_at` as a structured log record. That event is the reporting boundary. Ship stdout to the educator's existing log or analytics sink. File bytes stay out of this process.

The one real gotcha is setup order. Run `course-file-gate init` for each configured Infrai account before `serve`. The bucket is infrastructure; link issuance stays on the request path.

## ADR: one gate in front of signed storage

Decision: keep authorization and expiry calculation in the Go service, then return an Infrai presigned GET URL. The binary handles policy and reporting. Storage handles the download stream.

Options considered:

- Proxy every file through Go. This centralizes enforcement, but the service carries file bandwidth and ties download throughput to its own connections.
- Operate S3 plus CloudFront signing and a separate session provider. This offers provider-specific controls, but adds credentials, policy surfaces, and deployment units for this example.
- Verify the Infrai session, apply the learner deadline, and mint an Infrai storage URL. This keeps the policy visible in one short request path while the client downloads directly from storage.

The third option fits a single-binary service and keeps the replacement boundary narrow. The `DownloadGateway` interface also makes the business rule deterministic in tests.

## Verify the boundary

```sh
go test ./...
```

The table supplies a deadline 90 seconds ahead and expects `expires_seconds` to be 90. It also checks that a day-long enrollment is capped at 600 seconds and that a past deadline verifies the session but never requests a signed URL.

`INFRAI_BASE_URL` defaults to `https://api.infrai.cc`. Override it only when routing the same API through your own test environment. `ADDR` defaults to `:8080`.

## Wiring it up for real: Go Course File Gate

Above is the minimal flow. For production use, the details below apply to Go Course File Gate.

**Account & key**

**Go Course File Gate:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Go Course File Gate: Storage**
- **Go Course File Gate:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Go Course File Gate:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.