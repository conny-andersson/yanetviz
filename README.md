# Yanetviz

A local web application for documenting a home network on floor plans, with
optional live Wi-Fi observations from MikroTik WiFi CAPsMAN. The intended full application is
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

In **Topology & cables**, use **Sort cables** to choose original order, label
A–Z, or label Z–A. Alphabetic sorting ignores case, sorts embedded numbers
naturally, and places unlabeled cables last. It only changes the display order.

To document a rack, select a cabinet and set **Cabinet height (U)**. Select its
equipment and enter **Equipment height (U)** and **Starting unit (bottom of
equipment)**. U1 is at the bottom: a 2 U device starting at U3 occupies U3–U4.
The cabinet panel shows a rack layout and identifies equipment whose mounting
position is still unassigned. Blank values mean unknown height or unassigned position;
existing plans start with these fields unknown. Whole numbers from 1 to 1000
are supported. Save rejects overlapping equipment and equipment extending beyond
a known cabinet height. With an unknown cabinet height, capacity cannot yet be
checked. The initial layout assumes full-width equipment. Changing a device's
cabinet, removing its cabinet, or making it mobile clears its starting unit but
retains its equipment height.

Cabinets and equipment both have an optional **Short name** for cable labels;
blank values use the full name. In Edit mode, select **Auto-label cables…** in the
toolbar, or **Auto-label cable…** in a cable's details. Review the proposed labels,
select which to apply, then Save. Existing labels are kept unless **Allow
replacement of existing labels** is enabled. Apply changes can be undone.

Labels use `R01:U10:P05-R01:U12:P05` for cabinet equipment and
`R01:U12:P05-PumpRoom:P01` or `PumpRoom:P01-ds01:P01` for standalone equipment.
The starting rack unit identifies multi-unit equipment, and endpoint order stays
A-to-B. Rack-unit numbers keep the U prefix and are padded to at least two
digits: unit 3 becomes `U03`, for example `R01:U03:P05`. Simple port names such as `ether5` or `Port5` supply P05 automatically.
SFP names such as `sfp1`, `SFP+1`, and `sfp-sfpplus1` supply `Psfp01`, keeping
them distinct from copper `P01`. For example, a fiber label can be
`R01:U10:Psfp01-R02:U12:Psfp01`. For custom names, set **Cable label port
number** in the port details; an SFP connector preserves the Psfp prefix. Numbers
need to be unique within each port type on the equipment. Front/rear passive
terminals add F/R, such as P01F/P01R. Missing rack positions or port numbers and ambiguous
labels are reported in the preview. Choose short names that are distinct and easy
to read on printed labels. Renaming or moving equipment leaves existing cable
labels intact until explicitly regenerated.

Changes stay in the browser until Save. **Undo** retains the latest 30 edits.
Save errors retain the working copy. If another tab saved first, download your
working copy before reloading and reconciling the changes. The application does
not silently merge competing edits. Navigation away from pending edits prompts
in browsers that support the standard unsaved-changes warning.

## Client interface identities

Select equipment to inspect **Network interfaces**. In Edit mode, add an interface
and document its name, **Network identity domain**, and **MAC aliases** (one per
line). A physical device can have separate wired and Wi-Fi interfaces. Use the
same domain for sources observing the same LAN; the initial household import uses
`home`. MACs are normalized on save and must be unique within a domain. Adding a
known randomized MAC as an alias is an explicit manual decision; client IDs,
hostnames, and IPs never merge devices automatically.

Use **Physical port (optional)** to link an interface to one of that equipment's
documented ports. Leave it blank for Wi-Fi or an unknown mapping. Multiple logical
interfaces can share a port. Port details list the linked interface names. When
removing an uncabled port, the confirmation explains that its interface links
will be cleared; the interface identities and DHCP observations remain intact.

Each interface can show saved DHCP evidence: source/server, original client ID,
observed MAC/IP/hostname, lease status, and first/last collection times. Client IDs
remain opaque, optional values. An observation is a **snapshot**, not evidence
that the client is currently online, connected wirelessly, or served by a
particular AP. Different client IDs and DHCP sources retain separate evidence;
older samples cannot overwrite newer IP/hostname observations. Collection does
not rename equipment, move markers, or change documented cabling.

Inventory search also matches interface MACs and observed IPs, hostnames, and
client IDs. Use **Reload** to refresh saved observations. DHCP evidence is not refreshed automatically yet; Wi-Fi polling is configured separately below. Old import notes remain as historical documentation; structured
identities and DHCP evidence are maintained separately.

The existing Basement, Entrance floor, Attic, and Garage SVGs are bundled as
backgrounds. Replace a background with a PNG/JPEG up to 8 MiB and 64 megapixels;
review the preview before saving. Coordinates remain normalized to the image,
so a different crop may require repositioning. Each floor plan automatically fits
the available canvas, preserving its aspect ratio, when you switch floors or
resize the window. Zoom using the canvas controls and scroll to navigate a larger
plan; percentages are relative to the fitted view. Select **Fit** to restore the
entire plan. The editor currently targets desktop screens.

A new database currently includes built-in starter equipment, cabinet groups, and
cable drafts. This prototype seed is installation-specific; see
`internal/plan/seed.go`. Existing databases retain their own saved documentation,
and no credentials are imported from `inventory.local.yaml`.

Keep deployment-specific inventory, collected observations, and approved
management transports in private local files. `DESIGN.local.md`, when present,
contains installation notes and is excluded from Git. `DESIGN.md` describes the
application independently of any particular installation.

## WiFi CAPsMAN collection

Optional read-only collection supports the modern RouterOS
[`/interface/wifi` CAPsMAN tables](https://manual.mikrotik.com/docs/wireless/wifi/).
Configure the controller as the registration source; managed APs do not need
individual registration polling. Legacy `/caps-man` and `/interface/wireless`
collectors are not implemented in this increment.

Copy `wifi.example.json` to a private location and replace its example addresses
and UUIDs. Device UUIDs are available in a working-copy plan download. Create a
separate password file readable only by the application user. Set `passwordFile`
to its path, relative to the configuration file or absolute. Keep configuration
and passwords outside source control and portable plan exports. The application
does not parse `inventory.local.yaml` or expose credentials through its API.

```sh
./yanetviz -listen 127.0.0.1:8080 -data ./data -wifi-config /path/to/wifi.json
```

Omit `-wifi-config` to disable collection. HTTPS is the default and verifies the
system trust store; redirects and silent HTTP fallback are refused. An explicitly
approved HTTP source can use `transport: "rest-http"`. This sends Basic credentials
unencrypted and must be an intentional installation decision. Only fixed GET
requests with non-secret property allowlists are sent to configured endpoints.
See the [RouterOS REST API contract](https://manual.mikrotik.com/docs/developer-guides/rest-api/).

Map each physical `radio-mac` to its documented AP UUID in `radios`. Verify these
MACs against that AP's inventory. Controller-local radios automatically map to
the controller device. CAP identity and interface names are evidence, not device
matching keys. Virtual interfaces inherit their explicit master radio. Unmapped
or missing radios remain unresolved; review mappings in **Wi-Fi collection** and
edit the configuration before restarting. Interface/radio joins refresh with
every poll to avoid reusing obsolete mappings after controller provisioning.

Polling defaults to 15 seconds, with configurable intervals from 5–30 seconds,
five-second request timeouts, up to eight independent sources, sequential requests
per source, and capped retry backoff with jitter. Wi-Fi collection runs without an
open browser. Backend/UI updates use Server-Sent Events, refreshing observations
within about five seconds of collection without replacing pending plan edits.

**Wi-Fi collection** shows source health, last success/attempt, radio mappings,
matched clients and unmatched MACs. Clients match only existing interface MAC
aliases within the configured identity domain. Polling never creates or renames
equipment, moves markers, infers cables, or adds identities. Select a client or AP
to inspect SSID, band, signal and AP receive/transmit link rates where available.
Rates are in Mbps, not internet throughput; missing measurements remain unknown.
Registrations retain their access-authorization flag; an unauthorized registration
is still radio evidence and is not treated as an empty table or an absent client.

Selecting a positioned client or AP draws dashed Wi-Fi associations. Cross-floor
wireless portals navigate between the documented locations. Stale associations
are muted and labeled. Mobile/unplaced clients remain in the inventory and AP
client list without an inferred position on the floor plan. Conflicting fresh
registrations remain explicit and are not drawn as a definitive AP connection.

A successful complete snapshot can mark a previously seen MAC **Not associated**
and retain its last-seen time. Errors, partial replies and unsupported menus never
establish absence. Observations become stale after 45 seconds without a successful
snapshot, after a live-update connection failure, or on application restart until
refreshed. Removing a source disables collection and leaves historical snapshots
stale. This does not assert that a device is powered off. No general online/offline
status, automatic DHCP refresh, or observed wired link/PoE status is provided yet.

For Docker, mount the config and password file read-only, then extend the command
with `-wifi-config /config/wifi.json` while retaining `-listen 0.0.0.0:8080 -data
/data`. Ensure the application user can read the mounted files and reach the
configured management endpoints.

## Drawing cable routes

In **Edit mode**, set the current floor's optional integer **Floor level** in the
canvas toolbar (for example basement `-1`, entrance `0`, attic `1`). Leave unknown
levels blank. Multiple areas can share a level; levels do not change tab order.

Select a cable to open **Cable route · A → B**. Double-click its line to insert a
bend point, then drag the point to follow the actual route. Double-click a bend
point, or focus it and press Delete, to remove it. Drag circular cross-floor
markers to the riser or penetration. Clicking in Edit mode selects without
moving or navigating. In View mode, clicking a marker opens the adjacent floor
section and keeps the cable selected.

Use **Insert intermediate floor before endpoint B** to choose a pass-through
floor explicitly. Levels between the endpoints are marked in the chooser;
shared levels never cause an area to be selected automatically. Each intermediate
section has two independent markers and its own bend points. Reorder intermediate
sections with the arrow buttons and remove them individually. Check **Route
documented on this floor** only after documenting its path; this is independent
of cable draft status and observed link status.

**Straighten section** removes its bends; **Reset floor markers** restores its
automatic portal positions. **Reset entire route** previews what will be removed
before confirmation. All route edits use the normal Undo and Save workflow.
Moving equipment within a floor moves only its anchored cable endpoint. Moving
it to another floor preserves the saved route and shows a warning to reset the
route explicitly. Level changes preserve all route geometry and confirmation.

Routes currently visit each floor plan at most once and do not branch. Geometry
is manual documentation, not measured length. Existing cables remain straight
with automatic markers until edited. Floor levels, route sections, markers and
bends are included in working-copy JSON and saved-plan exports.

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
Rack dimensions, floor levels and cable route sections are optional entity fields, so older databases and
plan exports remain readable without a SQL migration. Saving, exporting, and
restoring retain the new fields alongside the rest of the documentation.

SQLite schema version 2 adds interface identities and a separate DHCP observation
table. Existing schema-1 databases are upgraded transactionally without changing
their documentation or revision. Full database backups retain both identities
and observations. Portable plan JSON/ZIP exports include interface identities
and MAC aliases, but omit DHCP observations. Removing an interface, MAC alias,
domain binding, or observation source removes dependent evidence when the plan is
saved; the interface removal confirmation explains the affected observations.
Regular plan saves retain evidence for surviving identities.
Schema version 3 adds a nullable physical-port foreign key to interfaces and
backfills it from any existing reference. Upgrading schema 1 or 2 preserves the
plan revision and saved DHCP evidence. References are optional for older plans.

Schema version 4 adds a separate Wi-Fi source-snapshot table. Upgrading versions
1–3 preserves the manual plan revision, route geometry, interface identities and
DHCP evidence. Portable plan JSON/ZIP omits Wi-Fi observations and source secrets;
a full SQLite backup retains observation history. Back up before upgrading. To
roll back to an older binary, restore the pre-upgrade database while the app is
stopped; older binaries intentionally refuse schema 4. Preserve the private Wi-Fi
configuration and password files separately when restoring to another machine.

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
| `POST /api/cable-labels/preview` | Preview labels from a complete working plan without saving it |
| `GET /api/export` | Download the saved documentation archive |
| `GET /api/observations/wifi` | Derived source health, radio mappings and matched/unmatched Wi-Fi clients |
| `GET /api/observations/wifi/events` | Authenticated SSE observation updates; reconnect sends a complete view |
| `GET /api/observations/dhcp` | Read saved DHCP evidence separately from the manual plan |
| `POST /api/observations/dhcp` | Validate and atomically record a batch without changing the manual plan revision |
| `GET/POST/DELETE /api/session` | Session status, sign in, sign out |
| `GET /api/health` | Readiness including database access |

Save requires the current revision. An outdated revision returns HTTP 409;
invalid documentation returns 400. Mutations require `X-Yanetviz: 1`. API requests
are limited to 32 MiB; DHCP observation batches are limited to 8 MiB and 10,000
records. Plan format version remains 1 with an optional `interfaces` collection;
SQLite schema version is 4. Observation records must reference a documented source
device and an interface owning the observed domain/MAC. DHCP client IDs are
preserved as supplied and do not act as matching keys.

## Remaining design work

CAPsMAN Wi-Fi polling is implemented for the modern WiFi tables. Legacy collectors,
a source/mapping configuration UI, automatic DHCP refresh, and observed wired
link/PoE status remain future work. PoE mode, standard/type, and voltage can be documented on ports, but
no live power flow is inferred. The topology screen is currently a grouped
inventory and cable register; a schematic graph with editable layout, richer
path highlighting and live-update notifications remain future
work. Uploaded image files may move to a separate asset store in a later schema.
