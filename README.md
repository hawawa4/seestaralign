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
- Join the Seestar to your home Wi-Fi (station mode) so the LXC can reach it.
- Put it in **EQ mode** on the wedge and roughly polar align it with the app.

## Install (Debian/Ubuntu LXC)

```sh
# 1. the app
make linux-amd64                       # or linux-arm64 for a Pi; on your build machine
scp bin/seestaralign-linux-amd64 root@lxc:/usr/local/bin/seestaralign
scp deploy/seestaralign.service root@lxc:/etc/systemd/system/

# 2. ASTAP: the *command-line* build plus a star database, from
#    https://www.hnsky.org/astap.htm (Linux downloads):
#    - astap_command-line_version_Linux_amd64.zip  -> unzip astap_cli into /opt/astap/
#    - d50_star_database.deb (or d80)              -> dpkg -i d50_star_database.deb
#    The GUI "astap" build needs a display and will NOT work in a container.

# 3. start it
systemctl daemon-reload && systemctl enable --now seestaralign
```

Then browse to `http://<lxc-ip>:8080`.

## Install with Docker

The image contains the app, the headless `astap_cli` and a star database, and needs
nothing else:

```sh
docker compose up -d --build          # then browse to http://<host>:8080
```

Or without compose:

```sh
docker build -t seestaralign .
docker run -d --name seestaralign --network host -v seestaralign-data:/data \
  --restart unless-stopped seestaralign
```

- **Host networking (`--network host`) is required.** It is what lets `seestar.local`
  (mDNS) and Alpaca discovery (UDP broadcast) reach the LAN. With port mapping
  (`-p 8080:8080`) the app still runs, but you have to enter the Seestar's IP address.
- **Star database:** chosen at build time with `--build-arg ASTAP_DB=...`.

  | Value | Download | Notes |
  |---|---|---|
  | `d50` (default) | ~860 MB | Recommended for every Seestar. |
  | `d80` | ~1.25 GB | Denser. |
  | `d05` | ~97 MB | Small test builds only. Too sparse for reliable S50 solves. |
  | `none` | – | Mount your own database at `/opt/astap` instead. |

- **Platforms:** `linux/amd64` and `linux/arm64` (Raspberry Pi 4/5 with a 64-bit OS).
  Multi-arch builds work with `docker buildx build --platform linux/arm64 ...`.
- **Settings** persist in the `/data` volume.
- **Clock:** the container uses the host clock, so keep the host NTP-synced.

Container notes (both LXC and Docker):
- **Clock:** the container uses the host's clock. Keep the host NTP-synced, because one
  second of clock error is 15″ of sky rotation.
- **Networking:** use bridged networking, so mDNS (`seestar.local`) and Alpaca discovery
  reach the Seestar's LAN. The app has its own mDNS lookup, so avahi isn't needed. If
  the name still doesn't resolve, type the Seestar's IP address into the UI.

## Using it

1. Connect. The default address is `seestar.local`, and you can type an IP over it.
   Port *auto* tries Alpaca discovery first, then 32323. Camera, mount, model and site
   are read from the device. The S50/S30 and the Pro models can't be told apart from
   the sensor size alone, so check the model shown and override it if needed. If a solve
   only matches another model's scale, the app switches models automatically.
2. Point the scope **near the meridian, 30–60° up, away from the pole and the zenith**,
   with room for 40° of RA travel in the move direction (east by default).
3. Press **Start**. It takes three frames with two 20° moves between them, and then
   shows the error.
4. Turn the knobs following the arrows. The arrows assume you are standing behind the
   scope facing the pole. The numbers update every frame. Aim for under 1′.
   - Near the end you may need a small azimuth tweak after an altitude change. That's
     real: when the axis is off the meridian, the wedge's altitude pivot also moves
     azimuth slightly.

**Simulator mode** (checkbox in the Connect card) runs everything against a virtual
Seestar with a known polar error, plus on-screen knob buttons. Use it to try the UI
indoors.

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
make test      # unit, simulator, fake Alpaca server and fake ASTAP tests
make run       # http://127.0.0.1:8080, tick "Simulator" to play
make reference # regenerate astropy reference values (needs uv)
```

Layout:
- `internal/astro`: coordinate math
- `internal/polar`: axis fit and adjustment tracker
- `internal/alpaca`: client, discovery, mDNS
- `internal/solver`: ASTAP wrapper
- `internal/session`: the routine, the simulator and the state
- `internal/web`: HTTP/SSE server and the embedded UI

### Not yet verified on real hardware

The app follows the Alpaca standard, but these Seestar-specific points haven't been
confirmed yet:

- Whether `MoveAxis(0, rate)` turns the RA axis in EQ mode. NINA's TPPA relies on the
  same call, and the Dec-change check stops the run if it doesn't.
- Whether the camera supports binary `ImageBytes`. If not, the app falls back to JSON,
  which is slower.
- How the Pro models name their wide-angle camera. Devices named "wide" or "guide" are
  skipped, and you can pick a camera number in the settings file.
