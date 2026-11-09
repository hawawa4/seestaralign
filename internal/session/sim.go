package session

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"seestaralign/internal/astro"
	"seestaralign/internal/imaging"
	"seestaralign/internal/models"
	"seestaralign/internal/polar"
	"seestaralign/internal/solver"
)

// simRig is a virtual Seestar on a misaligned wedge. The mount works in its
// own frame (RA axis = axis), so like a real mount it reports a constant Dec
// while moving in RA, even though its axis is off the pole.
type simRig struct {
	mu       sync.Mutex
	lat, lon float64
	atm      astro.Atmosphere
	axis     astro.Vec // north-oriented RA axis, horizon frame
	e1       astro.Vec // mount "meridian" reference, perpendicular to axis
	ha, dec  float64   // mount angles, rad
	tracking bool
	rate     float64 // current MoveAxis rate, deg/s
	last     time.Time
	azCum    float64
	model    models.Model
	rng      *rand.Rand
}

func newSimRig(set Settings) *simRig {
	lat, lon := set.SimLat, set.SimLon
	atm := astro.StandardAtmosphere
	pole := polar.TargetPole(lat, atm)
	pAlt, pAz := astro.VecToAltAz(pole)
	sign := 1.0
	if lat < 0 {
		sign = -1
	}
	vis := astro.AltAzToVec(pAlt+set.SimAltErrDeg*astro.Deg, pAz+sign*set.SimAzErrDeg*astro.Deg)
	axis := polar.VisibleAxis(vis, lat)
	r := &simRig{lat: lat, lon: lon, atm: atm, axis: axis, tracking: true, last: time.Now(),
		model: models.All[2], rng: rand.New(rand.NewPCG(1, 2))}
	m := astro.Vec{-math.Sin(lat * astro.Deg), 0, math.Cos(lat * astro.Deg)}
	r.e1 = m.Sub(axis.Scale(m.Dot(axis))).Unit()
	// start near the meridian, ~50° up
	startAz := 160.0
	if lat < 0 {
		startAz = 20
	}
	p := astro.AltAzToVec(50*astro.Deg, startAz*astro.Deg)
	r.dec = math.Pi/2 - astro.AngleBetween(axis, p)
	perp := p.Sub(axis.Scale(p.Dot(axis)))
	r.ha = math.Atan2(axis.Cross(r.e1).Dot(perp), r.e1.Dot(perp))
	return r
}

func (r *simRig) pointingAt(ha float64) astro.Vec {
	e2 := r.axis.Cross(r.e1)
	eq := r.e1.Scale(math.Cos(ha)).Add(e2.Scale(math.Sin(ha)))
	return eq.Scale(math.Cos(r.dec)).Add(r.axis.Scale(math.Sin(r.dec)))
}

func (r *simRig) advance(now time.Time) {
	dt := now.Sub(r.last).Seconds()
	r.last = now
	if r.tracking {
		r.ha += astro.SiderealRate * dt
	}
	// positive rate = towards the east = RA increasing = hour angle decreasing
	r.ha -= r.rate * astro.Deg * dt
}

func (r *simRig) Info() RigInfo {
	lat, lon := r.lat, r.lon
	m := r.model
	return RigInfo{
		BaseURL: "simulator", Simulated: true,
		Camera:    devSim("camera", "Simulated Seestar S50 Camera"),
		Telescope: devSim("telescope", "Simulated Seestar S50 Telescope"),
		SensorW:   m.Width, SensorH: m.Height, PixelUM: m.PixelUM,
		Lat: &lat, Lon: &lon, AlignmentMode: 1, CanSetTracking: true,
		AxisRates: []Rate{{0, 6}}, Detected: &m,
	}
}

func (r *simRig) Close() {}

func (r *simRig) Position(ctx context.Context) (float64, float64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.advance(now)
	ra := math.Mod(astro.LST(astro.JulianDate(now), r.lon)-r.ha+8*math.Pi, 2*math.Pi)
	return ra / astro.Deg / 15, r.dec / astro.Deg, nil
}

func (r *simRig) SetTracking(ctx context.Context, on bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.advance(time.Now())
	r.tracking = on
	return nil
}

func (r *simRig) MoveAxis(ctx context.Context, rate float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.advance(time.Now())
	r.rate = rate
	return nil
}

func (r *simRig) Slewing(ctx context.Context) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rate != 0, nil
}

func (r *simRig) SetGain(context.Context, int) error { return nil }
func (r *simRig) SyncClock(context.Context) error    { return nil }

// Knob turns the virtual wedge: positive alt raises the axis, positive az
// moves it east.
func (r *simRig) Knob(altArcmin, azArcmin float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.advance(time.Now())
	rotate := func(k astro.Mat) {
		r.axis, r.e1 = k.MulVec(r.axis), k.MulVec(r.e1)
	}
	if altArcmin != 0 {
		pivot := astro.Rotation(polar.ZAxis, r.azCum).MulVec(polar.EWAxis)
		beta := altArcmin / 60 * astro.Deg
		before, _ := astro.VecToAltAz(polar.VisibleAxis(r.axis, r.lat))
		after, _ := astro.VecToAltAz(polar.VisibleAxis(astro.Rotation(pivot, beta).MulVec(r.axis), r.lat))
		if (after > before) != (beta > 0) {
			beta = -beta
		}
		rotate(astro.Rotation(pivot, beta))
	}
	if azArcmin != 0 {
		alpha := azArcmin / 60 * astro.Deg
		if r.lat < 0 {
			alpha = -alpha // in the south, east is decreasing azimuth
		}
		rotate(astro.Rotation(polar.ZAxis, alpha))
		r.azCum += alpha
	}
}

func (r *simRig) Expose(ctx context.Context, seconds float64) (Frame, error) {
	r.mu.Lock()
	start := time.Now()
	r.advance(start)
	ha := r.ha
	if r.tracking {
		ha += astro.SiderealRate * seconds / 2
	}
	truth := r.pointingAt(ha)
	r.mu.Unlock()

	select {
	case <-ctx.Done():
		return Frame{}, ctx.Err()
	case <-time.After(time.Duration(seconds * float64(time.Second))):
	}
	return Frame{Image: r.starfield(truth, start), Start: start, Exposure: seconds, Truth: &truth}, nil
}

// starfield renders fake stars that move with the field, so the preview
// visibly shifts while you "turn the knobs".
func (r *simRig) starfield(p astro.Vec, t time.Time) imaging.Image {
	w, h := r.model.Width, r.model.Height
	img := imaging.Image{W: w, H: h, Pix: make([]float32, w*h)}
	for i := range img.Pix {
		img.Pix[i] = 800 + float32(r.rng.NormFloat64()*25)
	}
	ra, dec := astro.HorizonToJ2000(p, t, r.lat, r.lon, r.atm)
	scale := r.model.ScaleArcsec(0, 1) / 3600 // deg/px
	ox := ra * math.Cos(dec*astro.Deg) / scale
	oy := dec / scale
	field := rand.New(rand.NewPCG(7, 7))
	for i := 0; i < 220; i++ {
		x := math.Mod(field.Float64()*float64(w)*4+ox, float64(w)*4)
		y := math.Mod(field.Float64()*float64(h)*4+oy, float64(h)*4)
		if x < 0 {
			x += float64(w) * 4
		}
		if y < 0 {
			y += float64(h) * 4
		}
		if x >= float64(w) || y >= float64(h) {
			continue
		}
		amp := 2000 + 40000*math.Pow(field.Float64(), 4)
		for dy := -6; dy <= 6; dy++ {
			for dx := -6; dx <= 6; dx++ {
				xi, yi := int(x)+dx, int(y)+dy
				if xi < 0 || yi < 0 || xi >= w || yi >= h {
					continue
				}
				d2 := math.Pow(float64(xi)-x, 2) + math.Pow(float64(yi)-y, 2)
				img.Pix[yi*w+xi] += float32(amp * math.Exp(-d2/4.5))
			}
		}
	}
	return img
}

func devSim(typ, name string) devT { return devT{Type: typ, Number: 0, Name: name} }

// simSolver "plate solves" a simulated frame from its ground truth.
type simSolver struct{ rig *simRig }

func (s simSolver) Solve(ctx context.Context, f Frame, _ imaging.Image, _ *hint) (solver.Solution, string, error) {
	if f.Truth == nil {
		return solver.Solution{}, "", errors.New("no ground truth")
	}
	select {
	case <-ctx.Done():
		return solver.Solution{}, "", ctx.Err()
	case <-time.After(400 * time.Millisecond):
	}
	const noise = 0.5 * astro.Arcsec
	v := f.Truth.Add(astro.Vec{s.rig.rng.NormFloat64() * noise, s.rig.rng.NormFloat64() * noise, s.rig.rng.NormFloat64() * noise})
	ra, dec := astro.HorizonToJ2000(v, f.Mid(), s.rig.lat, s.rig.lon, s.rig.atm)
	return solver.Solution{RADeg: ra, DecDeg: dec, ScaleArcsec: s.rig.model.ScaleArcsec(0, 2), Seconds: 0.4}, "", nil
}
