# Generation v1

The public HTTP specification is `openapi/generation-v1.json`. Regenerate with
`go run ./cmd/generation-openapi ./openapi/generation-v1.json` from the module root.
Its schemas are derived from the Go DTOs and checked against the tracked artifact.

`POST /v1/jobs` requires Idempotency-Key, scope, Storage snapshotId, targetId and a
logical entrypoint. Optional parameters must be an object and must accompany a
nonempty logical parametersSchemaPath. Unknown HTTP fields and malformed JSON
are rejected synchronously. Draft 2020-12 schema validation runs asynchronously
against the immutable original snapshot; failure does not run Typst.

202 means the admission/reservation, activated snapshot pin and workflow-start
intent are durable. Receipts last seven days, scoped to workspace and initiator;
the response announces their expiry. Every replay checks current authorization.
Changed normalized inputs under an existing key conflict, including changes to
project scope or a numeric lexeme (`1`, `1.0` and `1e0` differ).

Normalization is `leafrun-json-v1`: keys sorted by UTF-8 bytes, array order kept,
no insignificant whitespace, strings encoded with Go encoding/json's
deterministic escaping, numeric lexemes preserved verbatim. Duplicate decoded
keys, invalid UTF-8/surrogates, nesting deeper than 64, out-of-int64 integer tokens
and nonfinite/out-of-range float tokens are rejected. Input and normalized bytes
are each bounded to 1 MiB. Float conversion is range checking only. Exact
normalized parameter bytes are persisted before external operations and passed
unchanged to schema validation and input.json derivation.

GET jobs and idempotent cancel require current access. Parameters and source/raw
diagnostics require source-read and otherwise are omitted. Cancellation serializes
with terminal success. A committed success remains historical after its PDF
expires 24 hours from first finalize. Generation's parameter/diagnostic retention
does not promise deletion of Storage's derived input.json on that schedule.

The SDK disables redirects, bounds JSON responses, transmits bearer credentials
only to the configured endpoint, and preserves normalized parameter lexemes.
Renderer framing is documented in `renderwire/v1/PROTOCOL.md`. Storage v1 owner
check DTOs remain the shared internal Access/Publishable/PinOwner interface.
