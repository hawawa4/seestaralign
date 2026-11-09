package polar

import (
	"math"
	"math/rand"
	"sort"
	"strings"
	"testing"
	"time"

	"seestaralign/internal/astro"
)

var t1 = time.Date(2026, 11, 9, 21, 0, 0, 0, time.UTC)

const w = astro.SiderealRate

func secs(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

// mountAxis builds a north-oriented "true" axis with a given error vs the target pole.
func mountAxis(lat, altErr, eastErr float64) astro.Vec {
	pAlt, pAz := astro.VecToAltAz(TargetPole(lat, astro.StandardAtmosphere))
	sign := 1.0
	if lat < 0 {
		sign = -1
	}
	v := astro.AltAzToVec(pAlt+altErr*astro.Deg, pAz+sign*eastErr*astro.Deg)
	return VisibleAxis(v, lat) // same flip both ways
}

// solved fakes a plate solve: physical pointing -> J2000 (+noise) -> back through the pipeline.
func solved(v astro.Vec, t time.Time, lat, lon, noise float64, rng *rand.Rand) astro.Vec {
	if noise > 0 {
		s := noise * astro.Arcsec
		v = v.Add(astro.Vec{rng.NormFloat64() * s, rng.NormFloat64() * s, rng.NormFloat64() * s}).Unit()
	}
	ra, dec := astro.HorizonToJ2000(v, t, lat, lon, astro.StandardAtmosphere)
	return astro.PointingVector(ra, dec, t, lat, lon, astro.StandardAtmosphere)
}

func simulate(lat, lon float64, axis astro.Vec, startAlt, startAz, step, noise float64, seed int64) ([3]astro.Vec, [3]time.Time) {
	rng := rand.New(rand.NewSource(seed))
	p1 := astro.AltAzToVec(startAlt*astro.Deg, startAz*astro.Deg)
	var pts [3]astro.Vec
	var ts [3]time.Time
	for i := 0; i < 3; i++ {
		dt := float64(i) * 40
		p := astro.Rotation(axis, w*dt+step*float64(i)*astro.Deg).MulVec(p1)
		ts[i] = t1.Add(secs(dt))
		pts[i] = solved(p, ts[i], lat, lon, noise, rng)
	}
	return pts, ts
}

func TestThreePointRecoversError(t *testing.T) {
	sites := []struct{ lat, lon, alt, az float64 }{
		{48.1, 11.6, 45, 170}, {40, -105, 30, 120}, {-33.9, 151.2, 50, 10}, {-33.9, 151.2, 40, 230},
	}
	errs := [][2]float64{{0.5, -0.3}, {-1.2, 2.0}, {0.05, 0.02}}
	for _, s := range sites {
		for _, e := range errs {
			axis := mountAxis(s.lat, e[0], e[1])
			pts, _ := simulate(s.lat, s.lon, axis, s.alt, s.az, 15, 0, 0)
			fitted, err := FitAxis(pts[0], pts[1], pts[2], s.lat)
			if err != nil {
				t.Fatal(err)
			}
			got := ComputeError(fitted, s.lat, TargetPole(s.lat, astro.StandardAtmosphere))
			if math.Abs(got.AltDeg-e[0])*3600 > 2 || math.Abs(got.AzDeg-e[1])*3600 > 2 {
				t.Errorf("site %+v err %v: got alt %.5f az %.5f", s, e, got.AltDeg, got.AzDeg)
			}
		}
	}
}

func TestNoiseSensitivity(t *testing.T) {
	lat, lon := 48.1, 11.6
	axis := mountAxis(lat, 0.3, 0.3)
	var errs []float64
	for seed := int64(0); seed < 25; seed++ {
		pts, _ := simulate(lat, lon, axis, 40, 180, 20, 2, seed)
		fitted, _ := FitAxis(pts[0], pts[1], pts[2], lat)
		e := ComputeError(fitted, lat, TargetPole(lat, astro.StandardAtmosphere))
		errs = append(errs, math.Hypot(e.AltDeg-0.3, e.AzDeg-0.3)*60)
	}
	sort.Float64s(errs)
	if med := errs[len(errs)/2]; med > 1 {
		t.Errorf("2\" solve noise with 20° steps gives median axis error %.2f', want < 1'", med)
	}
}

func TestDirections(t *testing.T) {
	cases := []struct {
		lat, alt, east      float64
		altDir, azDir, hand string
	}{
		{48, 0.5, 0.5, "lower", "west", "left"},
		{48, -0.5, -0.5, "raise", "east", "right"},
		{-35, -0.5, 0.5, "raise", "west", "right"},
		{-35, 0.5, -0.5, "lower", "east", "left"},
	}
	for _, c := range cases {
		e := ComputeError(mountAxis(c.lat, c.alt, c.east), c.lat, TargetPole(c.lat, astro.StandardAtmosphere))
		if e.AltDirection != c.altDir || e.AzDirection != c.azDir || e.AzHand != c.hand {
			t.Errorf("%+v: got %s/%s/%s", c, e.AltDirection, e.AzDirection, e.AzHand)
		}
	}
	// "east" in the south really means the axis lies towards az 90
	if v := VisibleAxis(mountAxis(-35, 0, 1), -35); v[1] <= 0 {
		t.Error("southern east error not towards east")
	}
}

func TestDegeneratePointsRejected(t *testing.T) {
	p := astro.AltAzToVec(0.5, 1)
	if _, err := FitAxis(p, p, p, 48); err == nil {
		t.Error("expected error")
	}
}

func TestAdjustmentTracksKnobTurns(t *testing.T) {
	for _, s := range []struct{ lat, lon, alt, az float64 }{{48.1, 11.6, 45, 170}, {-33.9, 151.2, 50, 10}} {
		axis := mountAxis(s.lat, 0.8, -0.6)
		pts, ts := simulate(s.lat, s.lon, axis, s.alt, s.az, 15, 0, 0)
		fitted, _ := FitAxis(pts[0], pts[1], pts[2], s.lat)
		tr := NewTracker(fitted, pts[2], ts[2], true)
		pole := TargetPole(s.lat, astro.StandardAtmosphere)

		// The user turns knobs over a few minutes while the mount keeps tracking
		// about its moving axis. The altitude pivot rides on the azimuth base.
		phys, trueAxis, now, aCum := pts[2], axis, ts[2], 0.0
		for k, turn := range [][2]float64{{0.2, -0.3}, {0.3, -0.25}, {0.1, -0.2}, {0, -0.05}} {
			const dt = 30.0
			phys = astro.Rotation(trueAxis, w*dt).MulVec(phys)
			pivot := astro.Rotation(ZAxis, aCum).MulVec(EWAxis)
			K := astro.Rotation(ZAxis, turn[0]*astro.Deg).Mul(astro.Rotation(pivot, turn[1]*astro.Deg))
			aCum += turn[0] * astro.Deg
			phys, trueAxis = K.MulVec(phys), K.MulVec(trueAxis)
			now = now.Add(secs(dt))

			res := tr.Update(solved(phys, now, s.lat, s.lon, 0, nil), now)
			got, want := ComputeError(res.Axis, s.lat, pole), ComputeError(trueAxis, s.lat, pole)
			if math.Abs(got.AltDeg-want.AltDeg)*3600 > 3 || math.Abs(got.AzDeg-want.AzDeg)*3600 > 3 {
				t.Errorf("lat %v step %d: got %.4f/%.4f want %.4f/%.4f", s.lat, k, got.AltDeg, got.AzDeg, want.AltDeg, want.AzDeg)
			}
			if res.ResidualArcsec > 5 || len(res.Warnings) > 0 {
				t.Errorf("step %d: residual %.1f warnings %v", k, res.ResidualArcsec, res.Warnings)
			}
		}
	}
}

func TestAdjustmentFlagsImplausibleMotion(t *testing.T) {
	lat, lon := 48.1, 11.6
	axis := mountAxis(lat, 0.5, 0.5)
	pts, ts := simulate(lat, lon, axis, 45, 170, 15, 0, 0)
	fitted, _ := FitAxis(pts[0], pts[1], pts[2], lat)
	tr := NewTracker(fitted, pts[2], ts[2], true)
	res := tr.Update(astro.Rotation(axis, 8*astro.Deg).MulVec(pts[2]), ts[2])
	if len(res.Warnings) == 0 || !strings.Contains(res.Warnings[len(res.Warnings)-1], "slew") {
		t.Errorf("expected slew warning, got %v", res.Warnings)
	}
}
