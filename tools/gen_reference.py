# /// script
# requires-python = ">=3.11"
# dependencies = ["astropy>=6"]
# ///
"""Generate astropy reference values for the Go astro tests.

    uv run tools/gen_reference.py > testdata/reference.json

Only needed when changing the coordinate math; the output is committed.
"""

import json
from datetime import datetime, timezone

from astropy import units as u
from astropy.coordinates import FK5, AltAz, EarthLocation, SkyCoord
from astropy.time import Time
from astropy.utils import iers

iers.conf.auto_download = False
iers.conf.auto_max_age = None

TIMES = ["2026-11-09T22:30:00Z", "2027-03-01T03:15:00Z", "2030-06-21T23:00:00Z"]
STARS = [(10, 20), (250, -40), (37.95, 89.26), (180, 0), (300, 60), (120, 70), (300, 5), (84, -1)]
SITES = [(48.1, 11.6), (-33.9, 151.2), (40.0, -105.0), (64.1, -21.9)]

out = {"precession": [], "altaz": [], "refraction": []}
for ts in TIMES:
    t = Time(datetime.fromisoformat(ts.replace("Z", "+00:00")).astimezone(timezone.utc))
    for ra, dec in STARS:
        c = SkyCoord(ra * u.deg, dec * u.deg, frame=FK5(equinox="J2000")).transform_to(FK5(equinox=t))
        out["precession"].append({"time": ts, "ra": ra, "dec": dec,
                                  "ra_now": c.ra.deg, "dec_now": c.dec.deg})
        for lat, lon in SITES:
            loc = EarthLocation(lat=lat * u.deg, lon=lon * u.deg, height=0 * u.m)
            aa = SkyCoord(ra * u.deg, dec * u.deg, frame="icrs").transform_to(AltAz(obstime=t, location=loc))
            if aa.alt.deg > 5:
                out["altaz"].append({"time": ts, "ra": ra, "dec": dec, "lat": lat, "lon": lon,
                                     "alt": aa.alt.deg, "az": aa.az.deg})

t = Time("2026-11-09T22:30:00")
loc = EarthLocation(lat=48 * u.deg, lon=11 * u.deg)
for alt in (15, 20, 30, 48, 60, 75):
    geo = SkyCoord(alt=alt * u.deg, az=0 * u.deg, frame=AltAz(obstime=t, location=loc))
    app = geo.transform_to(AltAz(obstime=t, location=loc, pressure=1013.25 * u.hPa,
                                 temperature=15 * u.deg_C, relative_humidity=0, obswl=0.55 * u.micron))
    out["refraction"].append({"alt": alt, "arcsec": (app.alt.deg - alt) * 3600})

print(json.dumps(out, indent=1))
