# seestaralign

Three-point polar alignment for ZWO Seestar telescopes (S30, S30 Pro, S50, S50 Pro)
in EQ mode on a wedge. It does the same job as NINA's TPPA plugin, without Windows.

It's a single static Go binary that serves a web page. Run it on any Linux box (an LXC,
a Raspberry Pi, a laptop) and open it from a phone or laptop browser on the same LAN.
It talks to the Seestar over **ASCOM Alpaca** and plate solves with **ASTAP**.

## How it works

1. Take a frame, plate solve it, and convert the result to where the optics physically
   point (alt/az at the exposure time, including refraction).
2. Turn **only the RA axis** (Alpaca `MoveAxis` on the primary axis) by 20°, then solve
   again. Repeat once more.
3. Those three positions lie on a circle around the mount's real RA axis. The circle's
   axis, compared with the celestial pole, gives the altitude and azimuth error.
4. **Adjust:** keep solving frames while you turn the knobs. Each frame is compared with
   point 3, using the model `P(t) = K · R(axis, sidereal·Δt) · P3`. Here `K` is the
   alt/az knob rotation, so the reading stays correct while the mount tracks. Each frame
   is solved against point 3, so errors don't pile up from frame to frame.

The math is covered by tests against astropy reference values, and by simulated full
runs in both hemispheres.

## Seestar preparation

- **Enable Alpaca** in the Seestar app. On recent firmware it is under
  *Me → Device Management → Assistive Features → Alpaca*. The Seestar's own Alpaca
  server listens on port **32323**. Note that `seestar_alp` *cannot* be used: it has no
  Camera device and its MoveAxis doesn't move the RA axis.
- Join the Seestar to your home Wi-Fi (station mode) so the machine running seestaralign can reach it.
- Put it in **EQ mode** on the wedge and roughly polar align it with the app.

## Build from source

You need **Go 1.26.3 or newer** (see `go.mod`) and **Node.js 22.12 or newer**. Node is
only used at build time, to compile the web UI (TypeScript + Preact + Vite) into static
files; the Go side has no third-party modules and no cgo. The UI is embedded in the
binary, so the result is still a single static file.

```sh
git clone https://github.com/hawawa4/seestaralign.git
cd seestaralign
just build                 # -> bin/seestaralign (for the machine you're on)
just linux-amd64           # -> bin/seestaralign-linux-amd64
just linux-arm64           # -> bin/seestaralign-linux-arm64 (Raspberry Pi, 64-bit OS)
just test                  # go vet + all tests; no hardware or ASTAP needed
```

Without [`just`](https://github.com/casey/just):

```sh
(cd web && npm ci && npm run build)     # writes internal/web/dist
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/seestaralign ./cmd/seestaralign
```

A plain `go build` without the first step still compiles, but the binary then answers
with "no web UI built in" instead of serving the page. The Docker image builds the UI
itself.

## Run locally

```sh
just run                   # builds, then serves http://127.0.0.1:8080
# or
bin/seestaralign -listen :8080        # reachable from other devices on the LAN
```

Open the page. To align a Seestar you also need ASTAP:

- Install the **command-line** build, `astap_cli`, and a star database from
  <https://www.hnsky.org/astap.htm> (Linux downloads):
  - `astap_command-line_version_Linux_amd64.zip` (or `_aarch64`): unzip `astap_cli`
    into `/opt/astap/`.
  - `d50_star_database.deb` (or `d80`): `dpkg -i d50_star_database.deb`.
- The app looks for `astap_cli` on `$PATH`, then in `/opt/astap/` and `/usr/local/bin/`.
  Star databases are found in `/opt/astap/`, `/usr/share/astap/data/` or next to the
  binary. Both can be set explicitly with `astap_path` / `astap_database_dir` in the
  settings file.
- The GUI `astap` build needs a display and will **not** work headless.

Flags:

| Flag | Default | Meaning |
|---|---|---|
| `-listen` | `:8080` | HTTP listen address. |
| `-config` | `$SEESTARALIGN_CONFIG`, else `$STATE_DIRECTORY/settings.json`, else the user config dir | Settings JSON file. |
| `-workdir` | `$TMPDIR/seestaralign` | Where `frame.fits` is written for ASTAP. |

### As a systemd service (Debian/Ubuntu, LXC, Raspberry Pi)

```sh
just linux-amd64                       # or linux-arm64; on your build machine
scp bin/seestaralign-linux-amd64 root@host:/usr/local/bin/seestaralign
scp deploy/seestaralign.service root@host:/etc/systemd/system/
# install astap_cli + a star database on the host as described above, then:
ssh root@host 'systemctl daemon-reload && systemctl enable --now seestaralign'
```

Then browse to `http://<host>:8080`. Settings live in `/var/lib/seestaralign/`.

For an LXC container, give it a bridged NIC on the same LAN as the Seestar, so mDNS
(`seestar.local`) and Alpaca discovery reach it. The app has its own mDNS lookup, so
avahi isn't needed. If the name still doesn't resolve, type the Seestar's IP address
into the UI.

## Run with Docker

The image contains the app, the headless `astap_cli` and the `d50` star database, and
needs nothing else. It is built for `linux/amd64` and `linux/arm64` (Raspberry Pi 4/5
with a 64-bit OS).

### Prebuilt image

A GitHub Action ([`.github/workflows/docker.yml`](.github/workflows/docker.yml)) runs
the tests and publishes the image to GitHub Container Registry on every push to `main`
and for every `v*` tag:

| Tag | Built from |
|---|---|
| `ghcr.io/hawawa4/seestaralign:latest` | the newest `main` or release tag |
| `ghcr.io/hawawa4/seestaralign:main` | the newest `main` |
| `ghcr.io/hawawa4/seestaralign:1.2.3`, `:1.2`, `:1` | release tag `v1.2.3` |
| `ghcr.io/hawawa4/seestaralign:sha-abc1234` | a specific commit |

Example `compose.yaml`:

```yaml
services:
  seestaralign:
    image: ghcr.io/hawawa4/seestaralign:latest
    container_name: seestaralign
    # Required: lets seestar.local (mDNS) and Alpaca discovery (UDP broadcast)
    # reach the LAN. A bridged container can't see the LAN's multicast.
    network_mode: host
    command: ["-listen", ":8080"]   # change the port here if 8080 is taken
    volumes:
      - seestaralign-data:/data     # settings.json
    restart: unless-stopped

volumes:
  seestaralign-data:
```

```sh
docker compose up -d       # then browse to http://<host>:8080
docker compose pull && docker compose up -d   # update to the newest image
```

Or without compose:

```sh
docker run -d --name seestaralign --network host \
  -v seestaralign-data:/data --restart unless-stopped \
  ghcr.io/hawawa4/seestaralign:latest
```

### Build the image yourself

The [`compose.yaml`](compose.yaml) in this repo builds from source:

```sh
docker compose up -d --build
# or
docker build -t seestaralign --build-arg ASTAP_DB=d50 .
docker buildx build --platform linux/arm64 -t seestaralign:arm64 .   # cross-build for a Pi
```

The star database is chosen at build time with `--build-arg ASTAP_DB=...`:

| Value | Download | Notes |
|---|---|---|
| `d50` (default) | ~860 MB | Recommended for every Seestar. |
| `d80` | ~1.25 GB | Denser. |
| `d05` | ~97 MB | Small test builds only. Too sparse for reliable S50 solves. |
| `none` | – | Mount your own database at `/opt/astap` instead. |

### Docker notes

- **Host networking (`--network host`) is required** for `seestar.local` and Alpaca
  discovery. With port mapping (`-p 8080:8080`) the app still runs, but you have to
  enter the Seestar's IP address.
- **Settings** persist in the `/data` volume.
- **Clock:** the container uses the host clock. Keep the host NTP-synced, because one
  second of clock error is 15″ of sky rotation.

## Using it

The page has four tabs: **Align** (checklist, progress, live error readout, last frame),
**Connect** (address, scan, device and model), **Settings** (grouped, collapsible
sections that save as you type) and **Log**. The night (red) theme keeps your eyes
dark-adapted, and Settings → Display has a dimmer that goes below your screen's minimum
brightness. While an alignment runs the screen is kept awake, when the browser allows
it (that needs `localhost` or https).

1. Connect. The default address is `seestar.local`, and you can type an IP over it.
   Port *auto* tries Alpaca discovery first, then 32323. Camera, mount, model and site
   are read from the device. The S50/S30 and the Pro models can't be told apart from
   the sensor size alone, so check the model shown and override it if needed. If a solve
   only matches another model's scale, the app switches models automatically.
2. Point the scope **near the meridian, 30–60° up, away from the pole and the zenith**,
   with room for 40° of RA travel in the move direction (east by default). Or let the
   app do it: in Settings, switch on *Slew here first* and set the
   position as Alt/Az (drag on the sky dome) or RA/Dec. It's saved with the other settings, and from then on
   every run begins by slewing there (alt/az is converted to RA/Dec for the slew; RA/Dec
   is in the frame the mount reports, JNow).
3. Press **Start**. It takes three frames with two 20° moves between them, and then
   shows the error.
4. Turn the knobs following the arrows. The arrows assume you are standing behind the
   scope facing the pole. The numbers update every frame. Aim for under 1′.
   - Near the end you may need a small azimuth tweak after an altitude change. That's
     real: when the axis is off the meridian, the wedge's altitude pivot also moves
     azimuth slightly.

### Accuracy

Noise in the 3-point fit falls with the *square* of the move distance. With ~1.4″ of
plate-solve noise:

| Move per step | Typical axis error |
|---|---|
| 10° (NINA's default) | ~1′ |
| 20° (default here) | ~0.3′ |
| 30° | ~0.13′ |

### Settings worth knowing

| Setting | Default | Notes |
|---|---|---|
| Exposure | 3 s | Shorter is fine if ASTAP finds enough stars. |
| Move per step | 20° | See the accuracy table above. |
| Direction | east | Flip it if the scope would hit something. |
| Refraction | on | Aligns to the apparent (refracted) pole, as NINA does by default. |
| Sync clock | on | The Seestar has no RTC. If its clock is off by more than 5 s on connect, the app sets it. |

## Safety checks

- The mount is always told to stop after a move, after an error, on Stop, and on shutdown.
- If **Dec changes** by more than 1° during an RA move, the run aborts. That means the
  mount isn't moving on RA alone, usually because it isn't really in EQ mode.
- If RA doesn't change within 6 s of a MoveAxis, the run aborts.
- During adjustment, a warning appears when the field is badly placed to separate the
  two knobs (near due east/west or the zenith), or when the field jumps more than knob
  turns plausibly could.

## Development

```sh
just test      # UI type-check + unit tests, then Go vet, simulated-mount, fake Alpaca and fake ASTAP tests
just dev       # hot-reloading UI on http://localhost:5173, Go API on 127.0.0.1:8080, settings in .dev/
just web       # build only the UI into internal/web/dist
just run       # build everything, then run the release binary
just reference # regenerate astropy reference values (needs uv)
```

Layout:
- `internal/astro`: coordinate math
- `internal/polar`: axis fit and adjustment tracker
- `internal/alpaca`: client, discovery, mDNS
- `internal/solver`: ASTAP wrapper
- `internal/session`: the routine and the state (the simulated mount used by the tests lives in `sim_test.go`)
- `internal/web`: HTTP/SSE server; embeds the built UI from `internal/web/dist`
- `web/`: the UI source (TypeScript, Preact, Vite). `src/types.ts` mirrors the server's JSON,
  `src/hooks/` holds the state, settings and theme logic, `src/components/` and `src/tabs/`
  the views (each with its own CSS), `src/lib/` the pure, unit-tested helpers

### Not yet verified on real hardware

The app follows the Alpaca standard, but these Seestar-specific points haven't been
confirmed yet:

- Whether `MoveAxis(0, rate)` turns the RA axis in EQ mode. NINA's TPPA relies on the
  same call, and the Dec-change check stops the run if it doesn't.
- Whether the Seestar accepts `SlewToCoordinatesAsync` (used for the starting position)
  in EQ mode. If the slew fails, the run stops with the error. Untick the option to
  carry on with manual pointing.
- Whether the camera supports binary `ImageBytes`. If not, the app falls back to JSON,
  which is slower.
- How the Pro models name their wide-angle camera. Devices named "wide" or "guide" are
  skipped, and you can pick a camera number in the settings file.
