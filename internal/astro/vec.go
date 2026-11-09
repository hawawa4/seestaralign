package astro

import "math"

// Vec is a 3-vector.
type Vec [3]float64

func (a Vec) Add(b Vec) Vec       { return Vec{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }
func (a Vec) Sub(b Vec) Vec       { return Vec{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func (a Vec) Scale(s float64) Vec { return Vec{a[0] * s, a[1] * s, a[2] * s} }
func (a Vec) Dot(b Vec) float64   { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
func (a Vec) Norm() float64       { return math.Sqrt(a.Dot(a)) }
func (a Vec) Neg() Vec            { return Vec{-a[0], -a[1], -a[2]} }
func (a Vec) Unit() Vec           { return a.Scale(1 / a.Norm()) }
func (a Vec) Cross(b Vec) Vec {
	return Vec{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

// AngleBetween is the numerically stable angle between two vectors, radians.
func AngleBetween(a, b Vec) float64 {
	return math.Atan2(a.Cross(b).Norm(), a.Dot(b))
}

// Mat is a 3x3 matrix, row-major.
type Mat [3][3]float64

func (m Mat) MulVec(v Vec) Vec {
	return Vec{
		m[0][0]*v[0] + m[0][1]*v[1] + m[0][2]*v[2],
		m[1][0]*v[0] + m[1][1]*v[1] + m[1][2]*v[2],
		m[2][0]*v[0] + m[2][1]*v[1] + m[2][2]*v[2],
	}
}

func (m Mat) Mul(n Mat) Mat {
	var r Mat
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			r[i][j] = m[i][0]*n[0][j] + m[i][1]*n[1][j] + m[i][2]*n[2][j]
		}
	}
	return r
}

func (m Mat) T() Mat {
	var r Mat
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			r[i][j] = m[j][i]
		}
	}
	return r
}

// Rotation is the right-handed (active) rotation by angle radians about axis.
func Rotation(axis Vec, angle float64) Mat {
	a := axis.Unit()
	x, y, z := a[0], a[1], a[2]
	c, s := math.Cos(angle), math.Sin(angle)
	C := 1 - c
	return Mat{
		{c + x*x*C, x*y*C - z*s, x*z*C + y*s},
		{y*x*C + z*s, c + y*y*C, y*z*C - x*s},
		{z*x*C - y*s, z*y*C + x*s, c + z*z*C},
	}
}
