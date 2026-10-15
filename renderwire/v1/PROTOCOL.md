# Renderer stream v1

Streams are private stdin/stdout pipes attached before container start. Disable
Docker logging for them. Never bind host project or output directories.

Every frame has a 48-byte header: ASCII `LRF1`, kind byte, three zero reserved
bytes, unsigned big-endian 64-bit payload length, and 32 raw SHA-256 bytes.
Kinds 1–7 are Request, Manifest, FileHeader, File, PDFHeader, PDF, Terminal.
E05 adds kinds 8–9: InputFileHeader and InputFile. Existing kinds retain their
numbers. The E05 capability must be deployed together with its controller.
The decoder checks size limits before allocating metadata. Binary payloads are
streamed, not allocated. Digests cover exact payload bytes. Unknown kinds,
reserved bits, wrong ordering, truncation, unconsumed payloads and extra bytes
after Terminal are errors. A decoder/encoder cannot resume after a stream error.

Input order: RenderRequest, base bundle, optional frozen input-file pairs,
InputTerminal, EOF. A bundle is a
manifest followed by each ordered project file's descriptor/data, followed by
each external font tree's files in manifest font order. In-project fonts reuse
the project bytes. Descriptors must match the validated M02 manifest exactly.
Empty directories occur in the manifest. Source native paths are metadata only.

E05 Request.Execution binds descriptor/execution digests, selected profile and
the owner's canonical PreparedInput. The child recomputes the descriptor from
the captured manifest/profile and verifies prepared bytes without applying
defaults again. InputFileHeader matches the representative PreparedFile exactly;
InputFile matches its size and SHA-256. PlanInputFiles orders generated paths and
streams one copy per path, including distinct copies for different extensions.
Every field reference remains separately bound to its authorized resource by
Generation. No caller identity or resource credentials enter the child.

Output: a derived bundle when derivation completed, PDFHeader/PDF only on
successful render, TerminalEnvelope, EOF. Validation failure emits only Terminal;
compile failure may retain the derived bundle for diagnostics without a PDF. Use
PeekKind to distinguish these paths. The terminal envelope declares the
exact attempt, scope, target, fingerprint, source/derived content identities,
engine version and PDF hash/size. Check these against the saved request and
verified staged bytes. Bound diagnostics and indicate truncation before encoding.

Limits: input/output 160/160 MiB including headers; each snapshot 100 MiB total
including external font bytes, at most 10,000 files, logical paths at most 4096
bytes; combined manifest/file-descriptor metadata 12 MiB; PDF 25 MiB; Request
12 MiB, Terminal 1 MiB; individual descriptor 16 KiB. Prepared JSON stays at
1 MiB, with at most 64 fileRefs and 25 MiB across unique generated input paths.
The metadata ceiling leaves
room for Storage's resource IDs and request envelope within its 16 MiB limit.

Receiving Terminal is insufficient to publish. Drain concurrently into private,
bounded staging; verify paths, sizes, identities and hashes; confirm the exact
container stopped with exit 0; only then upload/finalize. Discard partial/error
output. Unknown container status retains the execution slot and staging.

All renderer import, Core copies and PDF paths use /work tmpfs. Remove the import
copy after restore, close the base after derivation, and stream the derived
snapshot while it remains open. The controller already holds the output before
container exit removes tmpfs. No credentials or Docker socket enter the renderer.
