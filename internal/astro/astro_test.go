package astro

import (
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"
)

// Reference values come from astropy: tools/gen_reference.py.
type reference struct {
	Precession []struct {
		Time   string  `json:"time"`
		RA     float64 `json:"ra"`
		Dec    float64 `json:"dec"`
		RANow  float64 `json:"ra_now"`
		DecNow float64 `json:"dec_now"`
	} `json:"precession"`
	AltAz []struct {
		Time string  `json:"time"`
		RA   float64 `json:"ra"`
		Dec  float64 `json:"dec"`
		Lat  float64 `json:"lat"`
		Lon  float64 `json:"lon"`
		Alt  float64 `json:"alt"`
		Az   float64 `json:"az"`
	} `json:"altaz"`
	Refraction []struct {
		Alt    float64 `json:"alt"`
		Arcsec float64 `json:"arcsec"`
	} `json:"refraction"`
}

func loadRef(t *testing.T) reference {
	b, err := os.ReadFile("../../testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var r reference
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func mustTime(t *testing.T, s string) time.Time {
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return tt
}

func arcsecBetween(a, b Vec) float64 { return AngleBetween(a, b) / Arcsec }

func TestPrecessionMatchesAstropy(t *testing.T) {
	for _, c := range loadRef(t).Precession {
		jd := JulianDate(mustTime(t, c.Time))
		ra, dec := J2000ToJNow(c.RA*Deg, c.Dec*Deg, jd)
		if d := arcsecBetween(RADecToVec(ra, dec), RADecToVec(c.RANow*Deg, c.DecNow*Deg)); d > 0.5 {
			t.Errorf("%v %v@%s: off by %.2f\"", c.RA, c.Dec, c.Time, d)
		}
	}
}

func TestPointingMatchesAstropyAltAz(t *testing.T) {
	ref := loadRef(t)
	if len(ref.AltAz) < 20 {
		t.Fatal("too few reference cases")
	}
	for _, c := range ref.AltAz {
		v := PointingVector(c.RA, c.Dec, mustTime(t, c.Time), c.Lat, c.Lon, Atmosphere{})
		want := AltAzToVec(c.Alt*Deg, c.Az*Deg)
		// nutation/aberration/DUT1 are ignored: expect well under an arcminute
		if d := arcsecBetween(v, want); d > 45 {
			t.Errorf("%+v: off by %.1f\"", c, d)
		}
	}
}

func TestRefractionMatchesAstropy(t *testing.T) {
	for _, c := range loadRef(t).Refraction {
		got := Refraction(c.Alt*Deg, StandardAtmosphere) / Arcsec
		if math.Abs(got-c.Arcsec) > 10 {
			t.Errorf("alt %v: got %.1f\" want %.1f\"", c.Alt, got, c.Arcsec)
		}
	}
}

func TestSkyRotationSense(t *testing.T) {
	t0 := time.Date(2026, 11, 9, 22, 30, 0, 0, time.UTC)
	for _, lat := range []float64{48.1, -33.9} {
		dt := 600.0
		v0 := PointingVector(100, 10, t0, lat, 10, Atmosphere{})
		v1 := PointingVector(100, 10, t0.Add(time.Duration(dt*float64(time.Second))), lat, 10, Atmosphere{})
		r := Rotation(CelestialPoleVec(lat), SiderealRate*dt)
		if d := arcsecBetween(r.MulVec(v0), v1); d > 0.5 {
			t.Errorf("lat %v: sky rotation sense wrong, residual %.1f\"", lat, d)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	t0 := time.Date(2026, 11, 9, 22, 30, 0, 0, time.UTC)
	for _, atm := range []Atmosphere{{}, StandardAtmosphere} {
		v := AltAzToVec(35*Deg, 200*Deg)
		ra, dec := HorizonToJ2000(v, t0, 48, 11, atm)
		back := PointingVector(ra, dec, t0, 48, 11, atm)
		if d := arcsecBetween(v, back); d > 0.01 {
			t.Errorf("round trip off by %.3f\" (atm %+v)", d, atm)
		}
	}
}
