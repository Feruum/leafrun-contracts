# Renderer stream v1

Streams are private stdin/stdout pipes attached before container start. Disable
Docker logging for them. Never bind host project or output directories.

Every frame has a 48-byte header: ASCII `LRF1`, kind byte, three zero reserved
bytes, unsigned big-endian 64-bit payload length, and 32 raw SHA-256 bytes.
Kinds 1–7 are Request, Manifest, FileHeader, File, PDFHeader, PDF, Terminal.
The decoder checks size limits before allocating metadata. Binary payloads are
streamed, not allocated. Digests cover exact payload bytes. Unknown kinds,
reserved bits, wrong ordering, truncation, unconsumed payloads and extra bytes
after Terminal are errors. A decoder/encoder cannot resume after a stream error.

Input order: RenderRequest, base bundle, InputTerminal, EOF. A bundle is a
manifest followed by each ordered project file's descriptor/data, followed by
each external font tree's files in manifest font order. In-project fonts reuse
the project bytes. Descriptors must match the validated M02 manifest exactly.
Empty directories occur in the manifest. Source native paths are metadata only.

Output: a derived bundle when derivation completed, PDFHeader/PDF only on
successful render, TerminalEnvelope, EOF. Validation failure emits only Terminal;
compile failure may retain the derived bundle for diagnostics without a PDF. Use
PeekKind to distinguish these paths. The terminal envelope declares the
exact attempt, scope, target, fingerprint, source/derived content identities,
engine version and PDF hash/size. Check these against the saved request and
verified staged bytes. Bound diagnostics and indicate truncation before encoding.

Limits: input/output 128/160 MiB including headers; each snapshot 100 MiB total
including external font bytes, at most 10,000 files, logical paths at most 4096
bytes; combined manifest/file-descriptor metadata 12 MiB; PDF 25 MiB; Request
and Terminal each 1 MiB; individual descriptor 16 KiB. The metadata ceiling leaves
room for Storage's resource IDs and request envelope within its 16 MiB limit.

Receiving Terminal is insufficient to publish. Drain concurrently into private,
bounded staging; verify paths, sizes, identities and hashes; confirm the exact
container stopped with exit 0; only then upload/finalize. Discard partial/error
output. Unknown container status retains the execution slot and staging.

All renderer import, Core copies and PDF paths use /work tmpfs. Remove the import
copy after restore, close the base after derivation, and stream the derived
snapshot while it remains open. The controller already holds the output before
container exit removes tmpfs. No credentials or Docker socket enter the renderer.
