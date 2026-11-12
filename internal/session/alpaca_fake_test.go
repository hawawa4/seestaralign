package session

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"seestaralign/internal/astro"
)

// fakeSeestar is an Alpaca server backed by the simulator's mount physics.
type fakeSeestar struct {
	mu       sync.Mutex
	rig      *simRig
	expStart time.Time
	expDur   float64
	calls    []string
}

func (f *fakeSeestar) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.mu.Lock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	f.mu.Unlock()
	val := func(v any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"Value": v, "ErrorNumber": 0, "ErrorMessage": ""})
	}
	ok := func() { val(nil) }
	ctx := context.Background()
	switch strings.TrimPrefix(r.URL.Path, "/") {
	case "management/v1/configureddevices":
		val([]map[string]any{
			{"DeviceName": "Seestar S50 Wide Camera", "DeviceType": "Camera", "DeviceNumber": 1},
			{"DeviceName": "Seestar S50 Camera", "DeviceType": "Camera", "DeviceNumber": 0},
			{"DeviceName": "Seestar S50 Telescope", "DeviceType": "Telescope", "DeviceNumber": 0},
		})
	case "api/v1/camera/0/connected", "api/v1/telescope/0/connected":
		if r.Method == http.MethodPut {
			ok()
		} else {
			val(true)
		}
	case "api/v1/camera/0/name":
		val("Seestar S50 Camera")
	case "api/v1/camera/0/description", "api/v1/telescope/0/description":
		val("ZWO Seestar")
	case "api/v1/telescope/0/name":
		val("Seestar S50 Telescope")
	case "api/v1/camera/0/cameraxsize":
		val(64)
	case "api/v1/camera/0/cameraysize":
		val(48)
	case "api/v1/camera/0/pixelsizey":
		val(2.9)
	case "api/v1/telescope/0/sitelatitude":
		val(f.rig.lat)
	case "api/v1/telescope/0/sitelongitude":
		val(f.rig.lon)
	case "api/v1/telescope/0/alignmentmode":
		val(1)
	case "api/v1/telescope/0/cansettracking":
		val(true)
	case "api/v1/telescope/0/axisrates":
		val([]map[string]float64{{"Minimum": 0, "Maximum": 6}})
	case "api/v1/telescope/0/utcdate":
		if r.Method == http.MethodPut {
			ok()
		} else {
			val(time.Now().UTC().Format("2006-01-02T15:04:05.000Z"))
		}
	case "api/v1/telescope/0/rightascension":
		ra, _, _ := f.rig.Position(ctx)
		val(ra)
	case "api/v1/telescope/0/declination":
		_, dec, _ := f.rig.Position(ctx)
		val(dec)
	case "api/v1/telescope/0/tracking":
		_ = f.rig.SetTracking(ctx, r.Form.Get("Tracking") == "True")
		ok()
	case "api/v1/telescope/0/moveaxis":
		var rate float64
		fmt.Sscan(r.Form.Get("Rate"), &rate)
		if r.Form.Get("Axis") != "0" {
			http.Error(w, "bad axis", 400)
			return
		}
		_ = f.rig.MoveAxis(ctx, rate)
		ok()
	case "api/v1/telescope/0/slewtocoordinatesasync":
		var ra, dec float64
		fmt.Sscan(r.Form.Get("RightAscension"), &ra)
		fmt.Sscan(r.Form.Get("Declination"), &dec)
		_ = f.rig.SlewTo(ctx, ra, dec)
		ok()
	case "api/v1/telescope/0/abortslew":
		_ = f.rig.AbortSlew(ctx)
		ok()
	case "api/v1/telescope/0/slewing":
		s, _ := f.rig.Slewing(ctx)
		val(s)
	case "api/v1/camera/0/startexposure":
		f.mu.Lock()
		f.expStart = time.Now()
		fmt.Sscan(r.Form.Get("Duration"), &f.expDur)
		f.mu.Unlock()
		ok()
	case "api/v1/camera/0/imageready":
		f.mu.Lock()
		ready := time.Since(f.expStart).Seconds() >= f.expDur
		f.mu.Unlock()
		val(ready)
	case "api/v1/camera/0/imagearray":
		if r.Header.Get("Accept") != "application/imagebytes" {
			http.Error(w, "expected imagebytes", 400)
			return
		}
		const dx, dy = 64, 48
		buf := make([]byte, 44+dx*dy*2)
		le := binary.LittleEndian
		for i, v := range []int32{1, 0, 0, 0, 44, 2, 8, 2, dx, dy, 0} { // uint16 transmission
			le.PutUint32(buf[i*4:], uint32(v))
		}
		for i := 0; i < dx*dy; i++ {
			le.PutUint16(buf[44+2*i:], uint16(1000+i%50))
		}
		w.Header().Set("Content-Type", "application/imagebytes")
		_, _ = w.Write(buf)
	default:
		http.Error(w, "not implemented: "+r.URL.Path, 400)
	}
}

// fakeASTAP "solves" to the hint it was given, which is the mount's own
// idea of where it points (good enough to exercise the plumbing).
const fakeASTAP = `#!/bin/sh
while [ $# -gt 0 ]; do
  case "$1" in
    -f) f="$2"; shift;;
    -ra) ra="$2"; shift;;
    -spd) spd="$2"; shift;;
  esac
  shift
done
ini="${f%.*}.ini"
awk -v ra="$ra" -v spd="$spd" 'BEGIN { printf "PLTSOLVD=T\nCRVAL1=%.6f\nCRVAL2=%.6f\nCDELT2=0.00133\nCROTA2=12.5\n", ra*15, spd-90 }' > "$ini"
exit 0
`

func TestAlpacaPathEndToEnd(t *testing.T) {
	dir := t.TempDir()
	astap := filepath.Join(dir, "astap_cli")
	if err := os.WriteFile(astap, []byte(fakeASTAP), 0o755); err != nil {
		t.Fatal(err)
	}
	fake := &fakeSeestar{rig: newSimRig(defaultSim)}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	s := New(filepath.Join(dir, "settings.json"), dir)
	host := strings.TrimPrefix(srv.URL, "http://")
	patch, _ := json.Marshal(map[string]any{"host": host, "astap_path": astap,
		"exposure_s": 0.1, "move_rate": 10, "settle_s": 0,
		// the fake solves are unrefracted sky coordinates, so keep refraction out
		"refraction": false, "start_enabled": true, "start_mode": "radec", "start_ra_hours": 3.5, "start_dec_deg": 40})
	if err := s.UpdateSettings(patch); err != nil {
		t.Fatal(err)
	}
	if err := s.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	st := waitFor(t, s, time.Second, func(st *State) bool { return st.Phase == "ready" })
	if st.Rig.Camera.Number != 0 {
		t.Errorf("picked the wide camera: %+v", st.Rig.Camera)
	}
	if st.Model == nil || st.Model.Key != "s50" || st.Site == nil || st.Site.Source != "mount" {
		t.Fatalf("model/site not detected: %+v %+v", st.Model, st.Site)
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	st = waitFor(t, s, 30*time.Second, func(st *State) bool {
		return (st.Adjust != nil && st.Adjust.Frames >= 1) || len(st.Warnings) > 0
	})
	s.Stop()
	if len(st.Warnings) > 0 {
		t.Fatalf("warnings: %v", st.Warnings)
	}
	if st.Frame == nil || !st.Frame.Solved {
		t.Fatalf("frame not solved: %+v", st.Frame)
	}
	if st.Initial == nil {
		t.Fatal("no polar error computed")
	}
	// The fake solver echoes the mount's JNow coordinates as if they were
	// J2000, so the pipeline precesses them once more: the "error" must equal
	// the precession of the pole since J2000 (theta in IAU 1976).
	tc := (astro.JulianDate(time.Now()) - 2451545.0) / 36525
	theta := 2004.3109 * tc / 3600 // deg
	if d := math.Abs(st.Initial.SeparationDeg-theta) * 60; d > 0.5 {
		t.Errorf("separation %.2f′, want precession %.2f′", st.Initial.SeparationDeg*60, theta*60)
	}
	// The mount must have been stopped after each move, and at the end.
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.rig.rate != 0 {
		t.Error("mount left moving")
	}
	var moves, slews int
	for _, c := range fake.calls {
		if strings.HasSuffix(c, "/slewtocoordinatesasync") {
			slews++
		}
		if strings.HasSuffix(c, "/moveaxis") {
			moves++
		}
	}
	if slews != 1 {
		t.Errorf("expected one slew to the starting position, got %d", slews)
	}
	if moves < 4 { // 2 x (start, stop) at least
		t.Errorf("expected >= 4 moveaxis calls, got %d", moves)
	}
}
