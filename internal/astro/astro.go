// Package astro holds the coordinate math for polar alignment.
//
// Everything mount-related happens in the observer's horizon frame, which is
// fixed to the Earth (and therefore to the tripod and wedge):
//
//	x = north (horizon), y = east (horizon), z = zenith
//
// A direction with altitude alt and azimuth az (from north through east) is
// (cos alt cos az, cos alt sin az, sin alt).
//
// This frame is physically left-handed, so in these coordinates the sky, and a
// tracking mount, turns as a positive right-handed rotation about the
// north-celestial-pole vector: v(t+dt) = Rotation(ncp, +SiderealRate*dt) v(t).
//
// Plate solves give J2000 coordinates. To know where the telescope physically
// points we precess to the mean equinox of date, convert to alt/az at the
// exposure time and (optionally) add refraction, because the optics point at
// the apparent position of the field.
//
// Accuracy: IAU 1976 precession; nutation, aberration and DUT1 are ignored.
// Their effect on the derived mount axis is far below 1 arcminute.
package astro

import (
	"math"
	"time"
)

const (
	Deg    = math.Pi / 180
	Arcsec = Deg / 3600
	// SiderealRate is the Earth's rotation rate, rad/s.
	SiderealRate = 2 * math.Pi / 86164.0905
)

// Atmosphere controls the refraction model. Zero value = refraction off.
type Atmosphere struct {
	Enabled     bool
	PressureHPa float64
	TempC       float64
}

// StandardAtmosphere matches NINA's defaults (1013.25 hPa, 15 °C).
var StandardAtmosphere = Atmosphere{Enabled: true, PressureHPa: 1013.25, TempC: 15}

// ---------------------------------------------------------------- time

func JulianDate(t time.Time) float64 {
	return float64(t.UnixNano())/86400e9 + 2440587.5
}

// GMST is Greenwich mean sidereal time (IAU 1982), radians.
func GMST(jd float64) float64 {
	d := jd - 2451545.0
	tc := d / 36525
	g := 280.46061837 + 360.98564736629*d + 0.000387933*tc*tc - tc*tc*tc/38710000
	return math.Mod(math.Mod(g, 360)+360, 360) * Deg
}

// LST is local mean sidereal time, radians (longitude east positive).
func LST(jd, lonDeg float64) float64 {
	return math.Mod(GMST(jd)+lonDeg*Deg+4*math.Pi, 2*math.Pi)
}

// ---------------------------------------------------------------- spherical

func RADecToVec(ra, dec float64) Vec {
	cd := math.Cos(dec)
	return Vec{cd * math.Cos(ra), cd * math.Sin(ra), math.Sin(dec)}
}

func VecToRADec(v Vec) (ra, dec float64) {
	v = v.Unit()
	ra = math.Mod(math.Atan2(v[1], v[0])+2*math.Pi, 2*math.Pi)
	return ra, math.Asin(clamp(v[2]))
}

func AltAzToVec(alt, az float64) Vec {
	ca := math.Cos(alt)
	return Vec{ca * math.Cos(az), ca * math.Sin(az), math.Sin(alt)}
}

// VecToAltAz returns (alt, az) radians, az in [0, 2pi).
func VecToAltAz(v Vec) (alt, az float64) {
	v = v.Unit()
	return math.Asin(clamp(v[2])), math.Mod(math.Atan2(v[1], v[0])+2*math.Pi, 2*math.Pi)
}

func clamp(x float64) float64 { return math.Max(-1, math.Min(1, x)) }

// ---------------------------------------------------------------- precession

// PrecessionMatrix is IAU 1976, J2000 mean equator -> mean equator of date.
func PrecessionMatrix(jd float64) Mat {
	t := (jd - 2451545.0) / 36525
	zeta := (2306.2181*t + 0.30188*t*t + 0.017998*t*t*t) * Arcsec
	z := (2306.2181*t + 1.09468*t*t + 0.018203*t*t*t) * Arcsec
	theta := (2004.3109*t - 0.42665*t*t - 0.041833*t*t*t) * Arcsec
	cz, sz := math.Cos(zeta), math.Sin(zeta)
	cZ, sZ := math.Cos(z), math.Sin(z)
	ct, st := math.Cos(theta), math.Sin(theta)
	return Mat{
		{cZ*ct*cz - sZ*sz, -cZ*ct*sz - sZ*cz, -cZ * st},
		{sZ*ct*cz + cZ*sz, -sZ*ct*sz + cZ*cz, -sZ * st},
		{st * cz, -st * sz, ct},
	}
}

func J2000ToJNow(ra, dec, jd float64) (float64, float64) {
	return VecToRADec(PrecessionMatrix(jd).MulVec(RADecToVec(ra, dec)))
}

func JNowToJ2000(ra, dec, jd float64) (float64, float64) {
	return VecToRADec(PrecessionMatrix(jd).T().MulVec(RADecToVec(ra, dec)))
}

// ---------------------------------------------------------------- horizon

// EquatorialToHorizon takes a vector in the local hour-angle frame (x to
// HA=0,Dec=0; y to HA=-6h i.e. east; z to the NCP) into the horizon frame.
func EquatorialToHorizon(lat float64) Mat {
	s, c := math.Sin(lat), math.Cos(lat)
	// columns: meridian/equator point, east point, NCP
	return Mat{
		{-s, 0, c},
		{0, 1, 0},
		{c, 0, s},
	}
}

func HADecToVec(ha, dec float64) Vec {
	cd := math.Cos(dec)
	return Vec{cd * math.Cos(ha), -cd * math.Sin(ha), math.Sin(dec)}
}

// RADecNowToHorizon is the geometric horizon-frame vector of a JNow position.
func RADecNowToHorizon(ra, dec, jd, latDeg, lonDeg float64) Vec {
	ha := LST(jd, lonDeg) - ra
	return EquatorialToHorizon(latDeg * Deg).MulVec(HADecToVec(ha, dec))
}

// CelestialPoleVec is the geometric north celestial pole in the horizon frame
// (below the horizon in the southern hemisphere).
func CelestialPoleVec(latDeg float64) Vec {
	l := latDeg * Deg
	return Vec{math.Cos(l), 0, math.Sin(l)}
}

// ---------------------------------------------------------------- refraction

// Refraction for a true (geometric) altitude, Saemundsson (1986). Returns the
// amount in radians to add to get the apparent altitude.
func Refraction(trueAlt float64, atm Atmosphere) float64 {
	if !atm.Enabled {
		return 0
	}
	h := math.Max(trueAlt/Deg, -1)
	r := 1.02 / math.Tan((h+10.3/(h+5.11))*Deg) // arcmin
	r *= (atm.PressureHPa / 1010) * (283 / (273 + atm.TempC))
	return math.Max(r, 0) / 60 * Deg
}

func ApplyRefraction(v Vec, atm Atmosphere) Vec {
	alt, az := VecToAltAz(v)
	return AltAzToVec(alt+Refraction(alt, atm), az)
}

func RemoveRefraction(v Vec, atm Atmosphere) Vec {
	app, az := VecToAltAz(v)
	alt := app
	for i := 0; i < 10; i++ {
		alt = app - Refraction(alt, atm)
	}
	return AltAzToVec(alt, az)
}

// ---------------------------------------------------------------- the one-stop conversions

// PointingVector is where the optics physically point (horizon frame) given a
// plate-solved J2000 image centre and the mid-exposure time.
func PointingVector(raDeg, decDeg float64, t time.Time, latDeg, lonDeg float64, atm Atmosphere) Vec {
	jd := JulianDate(t)
	ra, dec := J2000ToJNow(raDeg*Deg, decDeg*Deg, jd)
	v := RADecNowToHorizon(ra, dec, jd, latDeg, lonDeg)
	if atm.Enabled {
		v = ApplyRefraction(v, atm)
	}
	return v.Unit()
}

// HorizonToRADecNow converts a geometric (unrefracted) horizon-frame vector
// into the JNow RA (hours) and Dec (degrees) a mount reports for it.
func HorizonToRADecNow(v Vec, t time.Time, latDeg, lonDeg float64) (raHours, decDeg float64) {
	ra, dec := horizonToJNow(v, JulianDate(t), latDeg, lonDeg)
	return ra / Deg / 15, dec / Deg
}

func horizonToJNow(v Vec, jd, latDeg, lonDeg float64) (ra, dec float64) {
	hv := EquatorialToHorizon(latDeg * Deg).T().MulVec(v.Unit())
	ha := math.Atan2(-hv[1], hv[0])
	dec = math.Asin(clamp(hv[2]))
	ra = math.Mod(LST(jd, lonDeg)-ha+4*math.Pi, 2*math.Pi)
	return ra, dec
}

// HorizonToJ2000 is the inverse of PointingVector; returns degrees.
func HorizonToJ2000(v Vec, t time.Time, latDeg, lonDeg float64, atm Atmosphere) (raDeg, decDeg float64) {
	if atm.Enabled {
		v = RemoveRefraction(v, atm)
	}
	jd := JulianDate(t)
	raNow, dec := horizonToJNow(v, jd, latDeg, lonDeg)
	ra, d := JNowToJ2000(raNow, dec, jd)
	return ra / Deg, d / Deg
}
