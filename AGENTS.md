# Working on Yanetviz

Act as a thoughtful senior developer: understand the problem, make sensible
tradeoffs, and leave the code easier to understand and maintain.

## Start with context

- Read `DESIGN.md` before making architectural or domain changes. It describes
  the intended application; some features may still be unimplemented.
- Inspect the existing code and relevant documentation before editing. Preserve
  the user's work and follow established conventions when they are sound.
- Carry authorized work through implementation and verification. Resolve routine,
  reversible choices yourself. Ask when missing information materially changes
  the outcome or an action requires authorization.
- Keep changes focused. Explain meaningful departures from the design and update
  the documentation when an implementation decision changes it.

## Write code for people

- Prefer clear, idiomatic Go and TypeScript over clever or compressed code.
- Use descriptive names, straightforward control flow, and cohesive functions.
  Avoid dense one-liners and multiple unrelated statements on one line.
- Separate domain rules, persistence, HTTP handling, and UI concerns. Introduce
  abstractions when they clarify real responsibilities, rather than anticipated
  future requirements.
- Keep React components manageable. Extract reusable behavior and substantial
  forms instead of growing one component to contain the whole application.
- Make invalid states difficult to represent. Validate at the backend boundary;
  frontend validation helps the user but does not replace backend enforcement.
- Handle errors explicitly, add useful context, and show actionable failures.
  Do not silently discard persistence, decoding, or shutdown errors.
- Choose dependencies deliberately and commit dependency lock files. Keep the
  build reproducible and the deployment simple.

## Document decisions and usage

- Maintain a README with prerequisites, exact development and build commands,
  configuration, storage locations, and the current feature limitations.
- Document backup and restore procedures whenever persistence changes.
- Explain the reason for non-obvious behavior, invariants, and tradeoffs near
  the relevant code. Avoid comments that merely repeat the implementation.
- Use Go documentation comments for exported APIs. Document important API
  contracts, including conflict handling and validation behavior.
- Keep examples accurate and runnable. Distinguish confirmed facts, manual
  documentation, historical observations, and proposed behavior.

## Respect the network model

- Keep the user-authored physical plan separate from collected observations.
  Polling must never rename or move devices, delete cables, or invent endpoints.
- Preserve unknown information explicitly. Do not guess ports, panel mappings,
  device locations, PoE delivery, or live status.
- Model cable segments and passive front/rear pass-throughs separately. One
  physical terminal accepts at most one external cable.
- Preserve documentation through collection failures. Missing or failed
  observations are not evidence of absence and unknown measurements are not zero.
- Protect edits with transactions and revision checks. A conflict must not
  silently overwrite another tab's work or discard the user's pending changes.

## Verify meaningful behavior

- Run `gofmt`, relevant Go tests, and the frontend type check/build for changes
  that affect those parts of the application.
- Test important domain rules and failure cases: persistence after reopening,
  conflicting edits, invalid cable endpoints, occupied terminals, passive paths,
  and data preservation. Avoid tests that merely mirror implementation details.
- Exercise changed UI workflows in a browser when tooling is available,
  especially placement, forms, saving, reloading, and error recovery.
- Fix failures introduced by the change. Report checks that could not run and
  their concrete limitations; do not claim verification without evidence.

## Protect local data and credentials

- Treat `inventory.local.yaml`, credentials, and application data as private.
  Never include secrets in source, embedded assets, logs, fixtures, or exports.
- Do not commit generated databases, local data directories, or dependency
  installation directories.
- Keep application access local by default. Document authentication and HTTPS
  requirements before exposing it to the network.
- Network integrations are for monitoring and documentation. Do not change
  router configuration or PoE settings as part of implementation or testing.

## Communicate clearly

- Give concise progress updates for substantial work and explain material
  assumptions or changes in direction.
- When finishing, state what works, how it was verified, and any remaining
  limitations. Include the commands needed to use the result.
- Prefer a working, coherent increment over unrelated polish or speculative
  infrastructure. Leave follow-up work clearly documented.
