# Yanetviz

A local web application for documenting a home network on floor plans. This first
increment focuses on the manual physical plan. The intended full application is
described in [DESIGN.md](DESIGN.md); contributor guidance is in [AGENTS.md](AGENTS.md).

## Run locally

Requires Go 1.25 or newer and Node.js 22.12 or newer (Node 24 is also supported).

```sh
make setup
make run
```

Open **http://127.0.0.1:8080**. The default listener is loopback only. Without
`make`, run these commands from the repository root:

```sh
(cd web && npm ci && npm run build)
go run -buildvcs=false ./cmd/yanetviz
```

The frontend must be built before compiling Go because its production assets are
embedded in the executable. `-buildvcs=false` also permits building in an IDE
workspace where Git metadata is unavailable.

## Start documenting

1. Switch from **View mode** to **Edit mode**.
2. Select a cabinet and choose **Place cabinet on floor plan**. Drag its marker
   to the correct position. Clicking an icon or its label only selects it;
   dragging starts after four pixels of pointer movement. Equipment in a cabinet
   shares that position.
3. Add equipment, set its name and role, then assign a cabinet or floor. Keep
   unknown locations unplaced; mobile equipment has no floor-plan position.
4. Add named ports. For a patch panel, wall socket, or fiber termination, add a
   **Position** to create separate front/rear terminals and an internal
   pass-through. Each position is independent of the others.
5. Add a cable, choose its two devices and known ports, and record its medium,
   label, optional length, and routing notes. Leave incomplete documentation as
   a draft. A terminal accepts only one external cable.
6. Use **Topology & cables** to browse equipment by floor and inspect the cable
   register. Selecting a cable shows its physical path through documented passive
   components and stops at unresolved terminations.
7. Select **Save changes**. Reloading the page restores the saved documentation.

Changes stay in the browser until Save. **Undo** retains the latest 30 edits.
Save errors retain the working copy. If another tab saved first, download your
working copy before reloading and reconciling the changes. The application does
not silently merge competing edits. Navigation away from pending edits prompts
in browsers that support the standard unsaved-changes warning.

The existing Basement, Entrance floor, Attic, and Garage SVGs are bundled as
backgrounds. Replace a background with a PNG/JPEG up to 8 MiB and 64 megapixels;
review the preview before saving. Coordinates remain normalized to the image,
so a different crop may require repositioning. Each floor plan automatically fits
the available canvas, preserving its aspect ratio, when you switch floors or
resize the window. Zoom using the canvas controls and scroll to navigate a larger
plan; percentages are relative to the fitted view. Select **Fit** to restore the
entire plan. The editor currently targets desktop screens.

A new database includes the confirmed devices and backbone described in the
design. Known cabinet membership and SW04 port labels are included, while exact
cabinet positions, AP locations, unknown ports, and intermediate panel routing
remain undocumented. No credentials are imported from `inventory.local.yaml`.

The working household database was enriched on 7 November 2026 with a one-time,
read-only RouterOS port inventory: RT01 has 11 physical Ethernet/SFP ports, SW01
28, SW02 6, and AP01/AP03/AP04 2 each. Existing documentation was preserved; SW04
retains its 10 manual ports. Each newly added port records its snapshot source
and time in its notes. This does not provide ongoing collection or determine
cable endpoints. The owner-approved HTTP REST transport for these six sources
is recorded in `DESIGN.md` and the private inventory; future work should honor
that per-source choice without silently downgrading other sources from HTTPS.

## Storage, export, and recovery

By default, SQLite lives in `data/yanetviz.db`. Select another directory with
`-data /path/to/data`. Run one application instance against a local filesystem.
Changes to all plan entities and the revision commit in one transaction.

**Export saved plan** downloads a ZIP containing `plan.json` and the bundled
floor-plan files referenced by the plan. Uploaded PNG/JPEG backgrounds are embedded
in the plan as data URLs in this first increment. **Download working copy** saves
JSON, including pending edits. Both exports contain manual documentation only.

To restore an export, unzip it if necessary, switch to Edit mode, and select
**Import plan** with `plan.json`. The backend validates the imported document
without saving it. Review the working copy and Save to replace the current plan.
The current application bundles the original SVG floor plans needed by these
exports. Arbitrary SVG uploads are intentionally not accepted.

For a full backup, stop the application and copy the **entire data directory**.
Restore that directory before starting a new instance. Do not copy only an open
SQLite main file: the application uses WAL, which may contain committed changes.
Back up configuration and passwords separately. Restore can be checked by
opening the floor plans and cable register in the new instance.

The initial schema uses relational foreign keys for ownership and endpoints,
with JSON payloads for each entity's descriptive fields. Full-plan replacement
keeps this small application's edit transaction simple. Later migrations can
extract fields as query requirements grow. Existing databases are never reseeded.

## Access and configuration

```sh
# Optional local login; required when listening outside loopback.
export YANETVIZ_PASSWORD='choose-a-long-unique-password'
./yanetviz -listen 127.0.0.1:8080 -data ./data
```

The password is supplied at startup and is never stored in the plan or returned
by the API. Sessions expire after 12 hours and after an application restart.
For access beyond the local machine, use a private HTTPS reverse proxy and pass
`-secure-cookie`. Keep the service private; it is not intended as a public hosted
service. Mutating requests require the application header and a same-origin
request; session cookies are HttpOnly and SameSite=Strict.

A Docker deployment is supplied:

```sh
export YANETVIZ_PASSWORD='choose-a-long-unique-password'
docker compose up --build -d
```

Compose binds to loopback and uses a persistent named volume. The container runs
as a non-root user. The Docker recipe has not yet been exercised in this workspace.

## Development and checks

```sh
make test                      # frontend build, Go race tests/vet, formatting
make format                    # gofmt and Prettier
(cd web && npx playwright install chromium)
make test-browser              # browser tests with a fresh temporary database
```

Browser tests start their own server on `127.0.0.1:18080` and remove their temporary
data when finished. They never use the normal `data/` directory.

For frontend development, first build its production assets once, then run
`go run -buildvcs=false ./cmd/yanetviz` and, in another terminal, `cd web && npm run dev`.
Vite proxies API requests and bundled floor plans to the Go server on port 8080.

The same-origin API exposes:

| Endpoint | Behavior |
| --- | --- |
| `GET /api/plan` | Current documentation and revision |
| `PUT /api/plan` | Validate and transactionally save a complete document |
| `POST /api/validate` | Validate an import without changing saved state |
| `GET /api/export` | Download the saved documentation archive |
| `GET/POST/DELETE /api/session` | Session status, sign in, sign out |
| `GET /api/health` | Readiness including database access |

Save requires the current revision. An outdated revision returns HTTP 409;
invalid documentation returns 400. Mutations require `X-Yanetviz: 1`. API requests
are limited to 32 MiB. Plan format version and SQLite schema version start at 1.

## Remaining design work

There is no MikroTik polling, live Wi-Fi association data, or observed link/PoE
status yet. PoE mode, standard/type, and voltage can be documented on ports, but
no live power flow is inferred. The topology screen is currently a grouped
inventory and cable register; a schematic graph with editable layout, richer
path highlighting, cable bend points, and live-update notifications remain future
work. Uploaded image files may move to a separate asset store in a later schema.
