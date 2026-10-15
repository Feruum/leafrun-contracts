# leafrun-contracts

## Agree on the shape of messages

This planned repository will define versioned API requests, WebSocket messages, and events shared between Leafrun components. For example, a render request and its result should mean the same thing to a browser and a server worker.

Services own their behavior; contracts describe what crosses their boundaries. Generated client code can later live here too.

**Status:** E03 introduces the Storage v1 OpenAPI specification, Go DTOs and HTTP client. It does not implement Access, Generation or Storage behavior.

**First roadmap step:** [LEA-18 — Define artifact IDs and API shapes](https://linear.app/bulnik/issue/LEA-18/e03-s3-rezultaty-i-ochistka). Linear may require workspace access; the public [roadmap](https://github.com/leafrun/leafrun/blob/main/docs/roadmap.md) explains the build order. See the [repository map](https://github.com/leafrun/leafrun/blob/main/docs/repositories.md) and [contribution guide](https://github.com/leafrun/leafrun/blob/main/CONTRIBUTING.md).

No project license has been selected yet.

## Storage v1

Import `github.com/leafrun/leafrun-contracts/storage/v1`. `NewClient(baseURL, bearerToken, httpClient)` returns a typed client for uploads, resources, snapshots, finalize, pins and artifact downloads. Mutation idempotency keys are part of the typed requests. The client does not retry mutations automatically and never follows redirects, including redirects from download endpoints. Close response bodies returned by `ResourceContent` and `ArtifactContent`.

`GetResource` reads authoritative scope, class, size and SHA-256 for a committed ready resource under current permissions. This metadata read is a point-in-time check; it does not retain the object. Acquire and activate its pin before committing a durable use, then verify the frozen descriptor when reading its bytes.

`openapi/storage-v1.json` is OpenAPI 3.1. Regenerate it from the DTO schemas with `go run ./cmd/openapi > openapi/storage-v1.json`; generated output must remain unchanged in verification. Private S3 keys/version IDs and database records are deliberately absent from the wire types. Owner check DTOs describe Access, Generation and pin-holder boundaries.

### Snapshot identity

`ValidateManifest` checks supported v1 manifest structure and attached M02 content ID. `ManifestContentID` hashes the original M02 canonical content projection with capitalized `FormatVersion`, `Files`, `Directories`, `Fonts` keys and ordered font trees. It does not hash wire JSON or provenance. Empty arrays are `[]`, not `null`. The Core manifest's content ID is independent of Storage's scoped, opaque snapshot ID.

Transport paths use logical `io/fs` naming, without backslashes or NUL bytes. A native Core materializer may reject names unsupported by its operating system. Transport validation does not impose Windows naming rules on snapshots captured on Linux. Resource bindings name each project or external-font file; in-project fonts reuse project resource bindings. Empty directories and ordered font metadata remain in the manifest.

### Authorization and lifecycle

The E03 server remains an internal dev service. Bearer credentials identify a service principal; arbitrary caller headers do not grant scope. Owner checks must fail closed. An artifact ID is not authorization, and finalize does not prove Generation has committed a publishable job. Every artifact metadata/content request checks access, owner publication, lifecycle and original expiry. Resource metadata/content requires current resource-read permission and lifecycle checks. PDF TTL is 24 hours from the original finalize; ordinary pins and retries do not extend it.

Pending pins precede the holder's durable pending reference. Activate only after that reference exists; ambiguous replies are resolved by reading the pin. Late activation cannot revive a deleting/expired object. Pin owner decisions distinguish `referenced`, `absent` and `unknown`.

Cleanup uses `ReleasePinBound` with the exact scope, object, holder and retention policy. Storage checks the authenticated holder's current access before returning a matching `released` or `absent` receipt. A missing or different binding returns `absent` without exposing another reference; HTTP errors, including `404`, do not confirm cleanup. The client rejects mismatched receipts. The original DELETE endpoint remains available for existing callers.

## E05 Projects and render selections

`projects/v1` defines revision registration, immutable technically validated
releases, publication compare-and-set, catalog/forms and fixed-service owner
resolution. Release creation never requires a successful Typst preview. Only
confirmed active Storage pins make pending releases usable; unpublishing keeps
rollback identity, while explicit deletion durably releases the final reference.
`openapi/projects-v1.json` is generated with `go run ./cmd/projects-openapi`.

`generation/v1.RenderSelectionRequest` selects either `releaseId` or
`snapshotId + targetId`. `CreateRender` calls `POST /v1/renders` with an
`Idempotency-Key` header. NormalizeSelection fingerprints the original selection;
Projects NormalizeExecution separately fingerprints frozen execution data.
Owner-prepared inputs and authoritative resource digests are never public request
fields. Parameters keep exact numeric lexemes. Defaults come from shared Core;
cache policy records intent and PDF reuse belongs to E15. Projects clients enforce
bounded strict responses, actor/selection/resource bindings and refuse redirects.
Additive input-file frames and their budgets are documented in
[`renderwire/v1/PROTOCOL.md`](renderwire/v1/PROTOCOL.md).

## Development

Use Go 1.27.0. Run `go test ./... -count=1`, `go vet ./...` and `go build ./...`; Linux additionally runs `go test -race ./... -count=1`. Tests cover M02 identity, malformed manifests, authenticated client errors/streams, and redirect refusal.

This feature is prepared for review. Related repositories may use an external temporary Go workspace while Contracts v0.1.0 is unreleased. No local `replace` or development `go.work` is shipped in this module; consumers must pin and verify the released version before their release.
