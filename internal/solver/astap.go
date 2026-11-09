// Package solver wraps the ASTAP plate solver.
package solver

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var exitCodes = map[int]string{
	1:  "no solution found",
	2:  "not enough stars detected (clouds? focus? exposure too short?)",
	16: "ASTAP could not read the image",
	32: "no star database found - install the D50 (or D80) database",
	33: "error reading the star database",
	34: "error updating the input file",
}

// ErrNoSolution is wrapped by Solve when ASTAP ran fine but found no match;
// the caller may retry with a different scale or wider search.
var ErrNoSolution = errors.New("no solution")

type Solution struct {
	RADeg       float64 `json:"ra_deg"`
	DecDeg      float64 `json:"dec_deg"`
	RotationDeg float64 `json:"rotation_deg"`
	ScaleArcsec float64 `json:"scale_arcsec"`
	Seconds     float64 `json:"seconds"`
}

type Options struct {
	Binary      string
	FOVDeg      float64  // image height (rows) in degrees
	RADeg       *float64 // hint
	DecDeg      *float64
	RadiusDeg   float64
	Database    string // e.g. "d50"; empty = ASTAP picks
	DatabaseDir string
	Timeout     time.Duration
}

// Find prefers the headless astap_cli; the GUI build needs a display.
func Find(configured string) string {
	cands := []string{}
	if configured != "" {
		cands = append(cands, configured)
	}
	cands = append(cands, "astap_cli", "/opt/astap/astap_cli", "/usr/local/bin/astap_cli", "astap", "/opt/astap/astap")
	for _, c := range cands {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

// Databases lists the star databases found next to ASTAP (best effort: the
// files are named like d50_0101.1476).
func Databases(binary, dir string) []string {
	dirs := []string{"/opt/astap", "/usr/share/astap/data"}
	if dir != "" {
		dirs = append([]string{dir}, dirs...)
	}
	if binary != "" {
		dirs = append(dirs, filepath.Dir(binary))
	}
	seen := map[string]bool{}
	var out []string
	for _, d := range dirs {
		matches, _ := filepath.Glob(filepath.Join(d, "*_*.*"))
		for _, m := range matches {
			base := filepath.Base(m)
			ext := filepath.Ext(base)
			if ext != ".1476" && ext != ".290" && ext != ".001" {
				continue
			}
			name := strings.ToLower(strings.SplitN(base, "_", 2)[0])
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	return out
}

func ParseINI(b []byte) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if ok {
			out[strings.ToUpper(strings.TrimSpace(k))] = strings.Trim(strings.TrimSpace(v), "'\" ")
		}
	}
	return out
}

// Solve plate-solves a FITS file.
func Solve(ctx context.Context, fitsPath string, o Options) (Solution, error) {
	if o.Binary == "" {
		return Solution{}, errors.New("ASTAP not found - install astap_cli")
	}
	args := []string{"-f", fitsPath, "-fov", fmt.Sprintf("%.4f", o.FOVDeg), "-z", "0", "-s", "500"}
	if o.RADeg != nil && o.DecDeg != nil {
		ra := *o.RADeg
		for ra < 0 {
			ra += 360
		}
		for ra >= 360 {
			ra -= 360
		}
		args = append(args, "-ra", fmt.Sprintf("%.6f", ra/15), "-spd", fmt.Sprintf("%.6f", *o.DecDeg+90),
			"-r", fmt.Sprintf("%.1f", o.RadiusDeg))
	} else {
		args = append(args, "-r", "180")
	}
	if o.Database != "" {
		args = append(args, "-D", o.Database)
	}
	if o.DatabaseDir != "" {
		args = append(args, "-d", o.DatabaseDir)
	}
	timeout := o.Timeout
	if timeout == 0 {
		timeout = 90 * time.Second
	}
	ini := strings.TrimSuffix(fitsPath, filepath.Ext(fitsPath)) + ".ini"
	_ = os.Remove(ini)

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	t0 := time.Now()
	cmd := exec.CommandContext(cctx, o.Binary, args...)
	out, runErr := cmd.CombinedOutput()
	dt := time.Since(t0).Seconds()
	if ctx.Err() != nil {
		return Solution{}, ctx.Err()
	}
	if cctx.Err() != nil {
		return Solution{}, fmt.Errorf("ASTAP timed out after %s", timeout)
	}

	vals := map[string]string{}
	if b, err := os.ReadFile(ini); err == nil {
		vals = ParseINI(b)
	}
	code := 0
	var ee *exec.ExitError
	if errors.As(runErr, &ee) {
		code = ee.ExitCode()
	} else if runErr != nil {
		return Solution{}, fmt.Errorf("running ASTAP: %w", runErr)
	}
	if code != 0 || vals["PLTSOLVD"] != "T" {
		reason := exitCodes[code]
		if reason == "" {
			reason = vals["ERROR"]
		}
		if reason == "" {
			s := strings.TrimSpace(string(out))
			if len(s) > 300 {
				s = s[len(s)-300:]
			}
			reason = fmt.Sprintf("exit code %d: %s", code, s)
		}
		if strings.Contains(strings.ToLower(string(out)), "cannot open display") {
			reason = "this is the ASTAP GUI build, which needs a display - install astap_cli"
		}
		if code == 1 {
			return Solution{}, fmt.Errorf("%s: %w", reason, ErrNoSolution)
		}
		return Solution{}, errors.New(reason)
	}
	ra, err1 := strconv.ParseFloat(vals["CRVAL1"], 64)
	dec, err2 := strconv.ParseFloat(vals["CRVAL2"], 64)
	if err1 != nil || err2 != nil {
		return Solution{}, errors.New("ASTAP reported success but gave no coordinates")
	}
	s := Solution{RADeg: ra, DecDeg: dec, Seconds: dt}
	s.RotationDeg, _ = strconv.ParseFloat(vals["CROTA2"], 64)
	if c, err := strconv.ParseFloat(vals["CDELT2"], 64); err == nil {
		if c < 0 {
			c = -c
		}
		s.ScaleArcsec = c * 3600
	}
	return s, nil
}
