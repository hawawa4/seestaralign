// Package polar implements three-point polar alignment.
//
// All vectors are unit vectors in the astro horizon frame (x north, y east,
// z up) and represent where the optics physically point: plate-solved
// positions converted to observed (refracted) alt/az at their own exposure
// time.
//
// Measurement: between the three frames only the RA axis turns (MoveAxis on
// the primary axis, plus sidereal tracking, which is also a rotation about the
// RA axis). The three pointing vectors therefore lie on a circle around the
// mount's RA axis, and the normal of the plane through them is that axis.
//
// Adjustment: let A0 be the measured axis and P3 the third pointing vector at
// t3. While the user turns the knobs the mount keeps tracking. Knob turns are
// rigid rotations K of the whole mount, and tracking is a rotation about the
// current axis K·A0. Because R(K·A0, θ) = K·R(A0, θ)·K⁻¹ the order of events
// doesn't matter, and at any later time
//
//	P(t) = K · R(A0, ω(t − t3)) · P3
//
// K is modelled as the two wedge knobs: K = Rz(α)·Ry(β), an altitude pivot
// about the east-west horizontal axis sitting on an azimuth base that turns
// about the vertical. Each plate solve gives P(t); we solve for the two angles
// and report the error of K·A0. Every frame is solved against P3 directly, so
// nothing accumulates from frame to frame.
package polar

import (
	"errors"
	"math"
	"time"

	"seestaralign/internal/astro"
)

var (
	ZAxis  = astro.Vec{0, 0, 1}
	EWAxis = astro.Vec{0, 1, 0}
)

var ErrPointsTooClose = errors.New("measurement points are too close together - did the RA axis actually move?")

// FitAxis returns the RA-axis direction from three pointing vectors, oriented
// towards the north celestial pole side (below the horizon in the south).
func FitAxis(p1, p2, p3 astro.Vec, latDeg float64) (astro.Vec, error) {
	a, b, c := p1.Unit(), p2.Unit(), p3.Unit()
	minChord := math.Min(b.Sub(a).Norm(), math.Min(c.Sub(b).Norm(), c.Sub(a).Norm()))
	n := b.Sub(a).Cross(c.Sub(b))
	if minChord < 0.5*astro.Deg || n.Norm() < 1e-12 {
		return astro.Vec{}, ErrPointsTooClose
	}
	n = n.Unit()
	if n.Dot(astro.CelestialPoleVec(latDeg)) < 0 {
		n = n.Neg()
	}
	return n, nil
}

// VisibleAxis flips the north-oriented axis to the pole you can see.
func VisibleAxis(axisNorth astro.Vec, latDeg float64) astro.Vec {
	if latDeg >= 0 {
		return axisNorth
	}
	return axisNorth.Neg()
}

// TargetPole is the visible celestial pole as the observed frame sees it. With
// refraction enabled this is the apparent (refracted) pole about which the
// stars appear to turn, which is NINA's default.
func TargetPole(latDeg float64, atm astro.Atmosphere) astro.Vec {
	alt := math.Abs(latDeg) * astro.Deg
	alt += astro.Refraction(alt, atm)
	az := 0.0
	if latDeg < 0 {
		az = math.Pi
	}
	return astro.AltAzToVec(alt, az)
}

// Error describes how far the mount axis is from the pole.
type Error struct {
	AltDeg        float64 `json:"alt_deg"`        // + : axis too high
	AzDeg         float64 `json:"az_deg"`         // + : axis east of the pole (raw azimuth)
	TotalDeg      float64 `json:"total_deg"`      // hypot(alt, az), like NINA
	SeparationDeg float64 `json:"separation_deg"` // true angular distance
	AxisAltDeg    float64 `json:"axis_alt_deg"`
	AxisAzDeg     float64 `json:"axis_az_deg"`
	PoleAltDeg    float64 `json:"pole_alt_deg"`
	PoleAzDeg     float64 `json:"pole_az_deg"`
	Northern      bool    `json:"northern"`
	AltDirection  string  `json:"alt_direction"` // "raise" | "lower"
	AzDirection   string  `json:"az_direction"`  // "east" | "west"
	AzHand        string  `json:"az_hand"`       // "left" | "right", facing the pole
}

func wrapDeg(a float64) float64 { return math.Mod(math.Mod(a+180, 360)+360, 360) - 180 }

// ComputeError compares a north-oriented axis with the target pole.
func ComputeError(axisNorth astro.Vec, latDeg float64, pole astro.Vec) Error {
	v := VisibleAxis(axisNorth, latDeg)
	aAlt, aAz := astro.VecToAltAz(v)
	pAlt, pAz := astro.VecToAltAz(pole)
	e := Error{
		AxisAltDeg: aAlt / astro.Deg, AxisAzDeg: aAz / astro.Deg,
		PoleAltDeg: pAlt / astro.Deg, PoleAzDeg: pAz / astro.Deg,
		Northern: latDeg >= 0,
	}
	e.AltDeg = e.AxisAltDeg - e.PoleAltDeg
	e.AzDeg = wrapDeg(e.AxisAzDeg - e.PoleAzDeg)
	if !e.Northern {
		// the south pole sits at az 180; azimuth decreasing there is towards the east
		e.AzDeg = -e.AzDeg
	}
	e.TotalDeg = math.Hypot(e.AltDeg, e.AzDeg)
	e.SeparationDeg = astro.AngleBetween(v, pole) / astro.Deg

	e.AltDirection = "raise"
	if e.AltDeg > 0 {
		e.AltDirection = "lower"
	}
	e.AzDirection = "east"
	if e.AzDeg > 0 {
		e.AzDirection = "west"
	}
	// facing the visible pole: in the north east is on the right, in the south on the left
	west := e.AzDirection == "west"
	if e.Northern == west {
		e.AzHand = "left"
	} else {
		e.AzHand = "right"
	}
	return e
}

// KnobRotation is the rigid mount rotation for azimuth knob alpha and altitude
// knob beta (radians). The altitude pivot rides on the azimuth base.
func KnobRotation(alpha, beta float64) astro.Mat {
	return astro.Rotation(ZAxis, alpha).Mul(astro.Rotation(EWAxis, beta))
}

// Adjustment is the tracker's view after a new plate solve.
type Adjustment struct {
	Axis           astro.Vec
	AlphaDeg       float64 // azimuth turned since the measurement
	BetaDeg        float64 // altitude turned since the measurement
	ResidualArcsec float64 // solver misfit (numerical sanity only)
	Conditioning   float64 // 0..1, smallest singular value of the jacobian
	Warnings       []string
}

const (
	// PoorConditioning: below this the field can't separate the two knobs well.
	PoorConditioning = 0.25
	// ImplausibleKnobDeg: bigger "knob turns" are more likely a slew or a bump.
	// A field-centre shift has two degrees of freedom, exactly like the two
	// knobs, so any shift fits; its size is the only sanity check we get.
	ImplausibleKnobDeg = 5.0
)

// Tracker turns successive plate solves into an updated mount axis.
type Tracker struct {
	Axis0    astro.Vec
	P3       astro.Vec
	T3       time.Time
	Tracking bool
	ab       [2]float64
}

func NewTracker(axis0, p3 astro.Vec, t3 time.Time, tracking bool) *Tracker {
	return &Tracker{Axis0: axis0.Unit(), P3: p3.Unit(), T3: t3, Tracking: tracking}
}

// Predicted is where the field would be at t if nobody touched the knobs.
func (tr *Tracker) Predicted(t time.Time) astro.Vec {
	if !tr.Tracking {
		return tr.P3
	}
	return astro.Rotation(tr.Axis0, astro.SiderealRate*t.Sub(tr.T3).Seconds()).MulVec(tr.P3)
}

func (tr *Tracker) Update(p astro.Vec, t time.Time) Adjustment {
	p = p.Unit()
	q := tr.Predicted(t)
	resid := func(a, b float64) astro.Vec { return KnobRotation(a, b).MulVec(q).Sub(p) }
	jac := func(a, b float64) (astro.Vec, astro.Vec, astro.Vec) {
		const h = 1e-7
		r := resid(a, b)
		return r, resid(a+h, b).Sub(r).Scale(1 / h), resid(a, b+h).Sub(r).Scale(1 / h)
	}

	a, b := tr.ab[0], tr.ab[1]
	for i := 0; i < 30; i++ {
		r, ja, jb := jac(a, b)
		// normal equations: (JᵀJ) step = -Jᵀr
		m11, m12, m22 := ja.Dot(ja), ja.Dot(jb), jb.Dot(jb)
		g1, g2 := -ja.Dot(r), -jb.Dot(r)
		det := m11*m22 - m12*m12
		if det == 0 {
			break
		}
		da := (m22*g1 - m12*g2) / det
		db := (m11*g2 - m12*g1) / det
		a, b = a+da, b+db
		if math.Hypot(da, db) < 1e-11 {
			break
		}
	}
	_, ja, jb := jac(a, b)
	m11, m12, m22 := ja.Dot(ja), ja.Dot(jb), jb.Dot(jb)
	// smallest eigenvalue of the 2x2 normal matrix -> smallest singular value
	tr2, det := m11+m22, m11*m22-m12*m12
	minEig := tr2/2 - math.Sqrt(math.Max(tr2*tr2/4-det, 0))
	cond := math.Sqrt(math.Max(minEig, 0))
	tr.ab = [2]float64{a, b}

	k := KnobRotation(a, b)
	out := Adjustment{
		Axis:           k.MulVec(tr.Axis0),
		AlphaDeg:       a / astro.Deg,
		BetaDeg:        b / astro.Deg,
		ResidualArcsec: astro.AngleBetween(k.MulVec(q), p) / astro.Arcsec,
		Conditioning:   cond,
	}
	if cond < PoorConditioning {
		out.Warnings = append(out.Warnings, "The current field is poorly placed to separate altitude from "+
			"azimuth (too close to due east/west or the zenith). Readings may be unreliable.")
	}
	if math.Max(math.Abs(out.AlphaDeg), math.Abs(out.BetaDeg)) > ImplausibleKnobDeg {
		out.Warnings = append(out.Warnings, "The field moved more than knob adjustments plausibly could. "+
			"Did the mount slew, stop tracking or get bumped? Consider restarting the alignment.")
	}
	return out
}
