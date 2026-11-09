package session

import (
	"context"
	"encoding/json"
	"math"
	"path/filepath"
	"testing"
	"time"

	"seestaralign/internal/astro"
	"seestaralign/internal/polar"
)

func waitFor(t *testing.T, s *Session, timeout time.Duration, cond func(st *State) bool) State {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		b, _ := s.Snapshot()
		var st State
		_ = json.Unmarshal(b, &st)
		if cond(&st) {
			return st
		}
		time.Sleep(50 * time.Millisecond)
	}
	b, _ := s.Snapshot()
	t.Fatalf("timed out; state: %s", b)
	return State{}
}

func runSim(t *testing.T, lat, lon, altErr, azErr float64) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, "settings.json"), dir)
	patch, _ := json.Marshal(map[string]any{
		"simulate": true, "exposure_s": 0.1, "move_rate": 10, "settle_s": 0,
		"sim_lat": lat, "sim_lon": lon, "sim_alt_err_deg": altErr, "sim_az_err_deg": azErr,
	})
	if err := s.UpdateSettings(patch); err != nil {
		t.Fatal(err)
	}
	if err := s.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	st := waitFor(t, s, 30*time.Second, func(st *State) bool { return st.Phase == "adjusting" || len(st.Warnings) > 0 })
	if len(st.Warnings) > 0 {
		t.Fatalf("warnings: %v", st.Warnings)
	}
	e := st.Initial
	// 0.5" simulated solve noise -> expect well under an arcminute
	if math.Abs(e.AltDeg-altErr)*60 > 0.8 || math.Abs(e.AzDeg-azErr)*60 > 0.8 {
		t.Fatalf("measured alt %.3f az %.3f, want %.3f %.3f", e.AltDeg, e.AzDeg, altErr, azErr)
	}
	if len(st.Points) != 3 {
		t.Fatalf("points: %d", len(st.Points))
	}
	// the three points should be ~20° of RA apart, i.e. well separated
	if math.Abs(st.Points[0].RADeg-st.Points[2].RADeg) < 30 {
		t.Errorf("points too close in RA: %+v", st.Points)
	}

	// Turn the knobs by the measured error. On a wedge the altitude pivot also
	// nudges azimuth when the axis is off-meridian, so compare against the
	// simulator's true axis rather than expecting exactly zero.
	if err := s.SimKnob(-e.AltDeg*60, -e.AzDeg*60); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	start := st.Adjust.Frames
	st = waitFor(t, s, 20*time.Second, func(st *State) bool { return st.Adjust != nil && st.Adjust.Frames >= start+2 })
	sr := s.rig.(*simRig)
	sr.mu.Lock()
	truth := polar.ComputeError(sr.axis, lat, polar.TargetPole(lat, astro.StandardAtmosphere))
	sr.mu.Unlock()
	if math.Abs(st.Error.AltDeg-truth.AltDeg)*60 > 1 || math.Abs(st.Error.AzDeg-truth.AzDeg)*60 > 1 {
		t.Errorf("after turning knobs: shown alt %.2f′ az %.2f′, truth alt %.2f′ az %.2f′",
			st.Error.AltDeg*60, st.Error.AzDeg*60, truth.AltDeg*60, truth.AzDeg*60)
	}
	if truth.TotalDeg*60 > 3 {
		t.Errorf("one correction should get within a few arcmin, truth is %.2f′", truth.TotalDeg*60)
	}
	for _, w := range st.Adjust.Warnings {
		t.Errorf("adjust warning: %s", w)
	}
}

func TestSimulatedAlignmentNorth(t *testing.T) { runSim(t, 48.1, 11.6, 0.8, -1.3) }
func TestSimulatedAlignmentSouth(t *testing.T) { runSim(t, -33.9, 151.2, -0.6, 0.9) }

func TestChooseRate(t *testing.T) {
	if r := chooseRate(3, nil); r != 3 {
		t.Error(r)
	}
	if r := chooseRate(3, []Rate{{0, 2}}); r != 2 {
		t.Error(r)
	}
	if r := chooseRate(3, []Rate{{0, 6}}); r != 3 {
		t.Error(r)
	}
	if r := chooseRate(3, []Rate{{4, 8}}); r != 4 {
		t.Error(r)
	}
}

func TestRADist(t *testing.T) {
	if d := raDistDeg(23.9, 0.1); math.Abs(d-3) > 1e-9 {
		t.Error(d)
	}
}
