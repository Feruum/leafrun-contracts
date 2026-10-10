# leafrun-contracts

Planned versioned API, WebSocket and event contracts with generated SDKs.

**Status: planned component.** This repository currently contains its scope and roadmap links. It does not yet provide a runnable service or package. The existing local rendering prototype lives in [leafrun-generation](https://github.com/leafrun/leafrun-generation).

## Responsibility

Versioned HTTP/WS/event/internal contracts and generated SDKs; no domain DB models.

## Boundaries

This is a versioned library, not a separately deployed service. Consumers pin compatible versions; domain rules and database ownership remain with their owning services.

## Roadmap

- [LEA-18 — S3 artifacts and garbage collection](https://linear.app/bulnik/issue/LEA-18/e03-s3-rezultaty-i-ochistka)
- [LEA-25 — Reliable asynchronous server rendering](https://linear.app/bulnik/issue/LEA-25/e04-nadyozhnyj-asinhronnyj-servernyj-render)
- [LEA-100 — User agents and helpers](https://linear.app/bulnik/issue/LEA-100/e13-polzovatelskie-agenty-i-helpery)
- [LEA-122 — Future capabilities: discovery](https://linear.app/bulnik/issue/LEA-122/e16-dalnejshie-vozmozhnosti-issledovanie)

See the [development roadmap](https://github.com/leafrun/leafrun/blob/main/docs/roadmap.md) for dependencies and acceptance milestones. Creating this repository does not complete its implementation issues.

## Contributing

Use English for code comments, documentation, examples, commit messages and pull requests. See the [contribution guide](https://github.com/leafrun/leafrun/blob/main/CONTRIBUTING.md) and [current architecture](https://github.com/leafrun/leafrun/blob/main/docs/architecture.md).

## License status

A project license has not been selected. No license is assigned by this repository scaffold.
