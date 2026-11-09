package solver

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func script(t *testing.T, body string) string {
	p := filepath.Join(t.TempDir(), "astap_cli")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func fitsFile(t *testing.T) string {
	p := filepath.Join(t.TempDir(), "frame.fits")
	_ = os.WriteFile(p, []byte("x"), 0o644)
	return p
}

func TestSolveSuccessAndArgs(t *testing.T) {
	bin := script(t, `echo "$@" > "$(dirname "$2")/args"
cat > "${2%.*}.ini" <<EOF
PLTSOLVD=T
CRVAL1= 83.8221
CRVAL2=-5.3911
CDELT2=-0.0013
CROTA2=91.5
EOF
`)
	f := fitsFile(t)
	ra, dec := 83.0, -5.0
	sol, err := Solve(context.Background(), f, Options{Binary: bin, FOVDeg: 0.72, RADeg: &ra, DecDeg: &dec, RadiusDeg: 10, Database: "d50"})
	if err != nil {
		t.Fatal(err)
	}
	if sol.RADeg != 83.8221 || sol.DecDeg != -5.3911 || sol.RotationDeg != 91.5 || sol.ScaleArcsec < 4.67 || sol.ScaleArcsec > 4.69 {
		t.Errorf("bad parse: %+v", sol)
	}
	args, _ := os.ReadFile(filepath.Join(filepath.Dir(f), "args"))
	for _, want := range []string{"-fov 0.7200", "-ra 5.533333", "-spd 85.000000", "-r 10.0", "-D d50"} {
		if !strings.Contains(string(args), want) {
			t.Errorf("args %q missing %q", args, want)
		}
	}
}

func TestSolveNoSolution(t *testing.T) {
	bin := script(t, `printf 'PLTSOLVD=F\nERROR=No solution found!\n' > "${2%.*}.ini"; exit 1`)
	_, err := Solve(context.Background(), fitsFile(t), Options{Binary: bin, FOVDeg: 1})
	if !errors.Is(err, ErrNoSolution) {
		t.Errorf("want ErrNoSolution, got %v", err)
	}
}

func TestSolveNoDatabase(t *testing.T) {
	bin := script(t, `exit 32`)
	_, err := Solve(context.Background(), fitsFile(t), Options{Binary: bin, FOVDeg: 1})
	if err == nil || errors.Is(err, ErrNoSolution) || !strings.Contains(err.Error(), "database") {
		t.Errorf("want database error, got %v", err)
	}
}

func TestSolveGUIBuild(t *testing.T) {
	bin := script(t, `echo "Gtk-WARNING **: cannot open display: " >&2; exit 1`)
	_, err := Solve(context.Background(), fitsFile(t), Options{Binary: bin, FOVDeg: 1})
	if err == nil || !strings.Contains(err.Error(), "astap_cli") {
		t.Errorf("want GUI-build hint, got %v", err)
	}
}
