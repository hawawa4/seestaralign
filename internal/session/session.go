// Package session runs the polar-alignment routine and owns the app state.
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"seestaralign/internal/alpaca"
	"seestaralign/internal/astro"
	"seestaralign/internal/imaging"
	"seestaralign/internal/models"
	"seestaralign/internal/polar"
	"seestaralign/internal/solver"
)

type devT = alpaca.Device

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

// Settings are persisted to disk and editable from the UI.
type Settings struct {
	Host         string   `json:"host"`
	Port         int      `json:"port"` // 0 = discover, then 32323
	CameraNum    int      `json:"camera_num"`
	TelescopeNum int      `json:"telescope_num"`
	Model        string   `json:"model"` // "" = auto
	Lat          *float64 `json:"lat"`   // overrides the mount's site
	Lon          *float64 `json:"lon"`

	ExposureS       float64 `json:"exposure_s"`
	Gain            *int    `json:"gain"`
	MoveDeg         float64 `json:"move_deg"`
	MoveRate        float64 `json:"move_rate"`
	Direction       string  `json:"direction"` // east | west
	SettleS         float64 `json:"settle_s"`
	SearchRadiusDeg float64 `json:"search_radius_deg"`

	// Starting position: when enabled, the first step of every run is a slew
	// here. The RA/Dec form is in the mount's own frame (JNow), the same
	// numbers the mount reports.
	StartEnabled bool    `json:"start_enabled"`
	StartMode    string  `json:"start_mode"` // altaz | radec
	StartAltDeg  float64 `json:"start_alt_deg"`
	StartAzDeg   float64 `json:"start_az_deg"`
	StartRAHours float64 `json:"start_ra_hours"`
	StartDecDeg  float64 `json:"start_dec_deg"`

	Refraction  bool    `json:"refraction"`
	PressureHPa float64 `json:"pressure_hpa"`
	TempC       float64 `json:"temp_c"`

	ASTAPPath        string `json:"astap_path"`
	ASTAPDatabase    string `json:"astap_database"`
	ASTAPDatabaseDir string `json:"astap_database_dir"`
	SyncClock        bool   `json:"sync_clock"`
}

func DefaultSettings() Settings {
	return Settings{
		Host: "seestar.local", CameraNum: -1, TelescopeNum: -1,
		ExposureS: 3, MoveDeg: 20, MoveRate: 3, Direction: "east", SettleS: 2, SearchRadiusDeg: 15,
		StartMode: "altaz", StartAltDeg: 50, StartAzDeg: 180,
		Refraction: true, PressureHPa: 1013.25, TempC: 15, SyncClock: true,
	}
}

type Point struct {
	N      int       `json:"n"`
	RADeg  float64   `json:"ra_deg"`
	DecDeg float64   `json:"dec_deg"`
	AltDeg float64   `json:"alt_deg"`
	AzDeg  float64   `json:"az_deg"`
	Time   time.Time `json:"time"`
}

type HistoryEntry struct {
	T     time.Time `json:"t"`
	Total float64   `json:"total"`
	Alt   float64   `json:"alt"`
	Az    float64   `json:"az"`
}

type LogEntry struct {
	T     time.Time `json:"t"`
	Level string    `json:"level"`
	Msg   string    `json:"msg"`
}

type FrameInfo struct {
	ID          int       `json:"id"`
	Time        time.Time `json:"time"`
	Solved      bool      `json:"solved"`
	Solving     bool      `json:"solving"`
	RADeg       float64   `json:"ra_deg"`
	DecDeg      float64   `json:"dec_deg"`
	ScaleArcsec float64   `json:"scale_arcsec"`
	SolveS      float64   `json:"solve_s"`
	Error       string    `json:"error,omitempty"`
}

type AdjustInfo struct {
	Frames       int      `json:"frames"`
	Conditioning float64  `json:"conditioning"`
	AlphaDeg     float64  `json:"alpha_deg"`
	BetaDeg      float64  `json:"beta_deg"`
	Warnings     []string `json:"warnings"`
}

type Site struct {
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Source string  `json:"source"` // mount | override | none
}

// State is everything the UI shows; it is sent whole on every change.
type State struct {
	Phase       string         `json:"phase"` // disconnected connecting ready measuring adjusting
	Step        string         `json:"step"`
	Settings    Settings       `json:"settings"`
	Rig         *RigInfo       `json:"rig"`
	Site        *Site          `json:"site"`
	Model       *models.Model  `json:"model"`
	ModelSource string         `json:"model_source"`
	Models      []models.Model `json:"models"`
	Points      []Point        `json:"points"`
	Initial     *polar.Error   `json:"initial"`
	Error       *polar.Error   `json:"error"`
	History     []HistoryEntry `json:"history"`
	Adjust      *AdjustInfo    `json:"adjust"`
	Frame       *FrameInfo     `json:"frame"`
	Warnings    []string       `json:"warnings"`
	Log         []LogEntry     `json:"log"`
	ASTAP       struct {
		Path      string   `json:"path"`
		Databases []string `json:"databases"`
	} `json:"astap"`
}

type hint struct {
	RA, Dec float64
	Wide    bool // search a much larger radius
}

type frameSolver interface {
	// Solve returns the solution and, if a different model's scale was what
	// worked, that model's key.
	Solve(ctx context.Context, f Frame, binned imaging.Image, h *hint) (solver.Solution, string, error)
}

type Session struct {
	mu           sync.Mutex
	st           State
	rig          Rig
	solver       frameSolver
	cancel       context.CancelFunc
	done         chan struct{}
	subs         map[chan struct{}]struct{}
	jpeg         []byte
	settingsPath string
	workDir      string
}

func New(settingsPath, workDir string) *Session {
	s := &Session{subs: map[chan struct{}]struct{}{}, settingsPath: settingsPath, workDir: workDir}
	s.st.Phase = "disconnected"
	s.st.Settings = DefaultSettings()
	s.st.Models = models.All
	if b, err := os.ReadFile(settingsPath); err == nil {
		set := DefaultSettings()
		if err := json.Unmarshal(b, &set); err == nil {
			s.st.Settings = set
		} else {
			log.Printf("ignoring bad settings file %s: %v", settingsPath, err)
		}
	}
	s.refreshASTAP()
	return s
}

// ---------------------------------------------------------------- state plumbing

func (s *Session) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}
}

// notifyLocked must be called with s.mu held.
func (s *Session) notifyLocked() {
	for ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (s *Session) update(f func(st *State)) {
	s.mu.Lock()
	f(&s.st)
	s.notifyLocked()
	s.mu.Unlock()
}

func (s *Session) Snapshot() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Marshal(&s.st)
}

func (s *Session) FrameJPEG() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jpeg
}

func (s *Session) logf(level, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[%s] %s", level, msg)
	s.update(func(st *State) {
		st.Log = append(st.Log, LogEntry{T: time.Now(), Level: level, Msg: msg})
		if len(st.Log) > 150 {
			st.Log = st.Log[len(st.Log)-150:]
		}
	})
}

func (s *Session) info(format string, args ...any) { s.logf("info", format, args...) }
func (s *Session) warn(format string, args ...any) { s.logf("warn", format, args...) }

func (s *Session) step(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	s.update(func(st *State) { st.Step = msg })
}

func (s *Session) settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st.Settings
}

func (s *Session) refreshASTAP() {
	set := s.st.Settings
	s.st.ASTAP.Path = solver.Find(set.ASTAPPath)
	s.st.ASTAP.Databases = solver.Databases(s.st.ASTAP.Path, set.ASTAPDatabaseDir)
}

// UpdateSettings merges a partial JSON object into the settings and saves them.
func (s *Session) UpdateSettings(patch []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	set := s.st.Settings
	if err := json.Unmarshal(patch, &set); err != nil {
		return err
	}
	if set.Direction != "east" && set.Direction != "west" {
		return errors.New(`direction must be "east" or "west"`)
	}
	if set.ExposureS <= 0 || set.ExposureS > 60 {
		return errors.New("exposure must be between 0 and 60 s")
	}
	if set.MoveDeg < 5 || set.MoveDeg > 60 {
		return errors.New("move distance must be between 5° and 60°")
	}
	if set.MoveRate <= 0 {
		return errors.New("move rate must be positive")
	}
	if set.StartMode != "altaz" && set.StartMode != "radec" {
		return errors.New(`start position mode must be "altaz" or "radec"`)
	}
	if set.StartAltDeg < 0 || set.StartAltDeg > 90 {
		return errors.New("start altitude must be between 0° and 90°")
	}
	if set.StartAzDeg < 0 || set.StartAzDeg >= 360 {
		return errors.New("start azimuth must be between 0° and 360°")
	}
	if set.StartRAHours < 0 || set.StartRAHours >= 24 {
		return errors.New("start RA must be between 0 h and 24 h")
	}
	if set.StartDecDeg < -90 || set.StartDecDeg > 90 {
		return errors.New("start Dec must be within ±90°")
	}
	if (set.Lat == nil) != (set.Lon == nil) {
		return errors.New("set both latitude and longitude, or neither")
	}
	if set.Lat != nil && math.Abs(*set.Lat) > 90 {
		return errors.New("latitude must be within ±90°")
	}
	s.st.Settings = set
	s.refreshASTAP()
	s.resolveSiteLocked()
	s.resolveModelLocked()
	s.notifyLocked()
	return s.saveLocked()
}

func (s *Session) saveLocked() error {
	if s.settingsPath == "" {
		return nil
	}
	b, _ := json.MarshalIndent(s.st.Settings, "", "  ")
	_ = os.MkdirAll(filepath.Dir(s.settingsPath), 0o755)
	return os.WriteFile(s.settingsPath, b, 0o644)
}

func (s *Session) resolveSiteLocked() {
	set := s.st.Settings
	switch {
	case set.Lat != nil && set.Lon != nil:
		s.st.Site = &Site{Lat: *set.Lat, Lon: *set.Lon, Source: "override"}
	case s.st.Rig != nil && s.st.Rig.Lat != nil:
		s.st.Site = &Site{Lat: *s.st.Rig.Lat, Lon: *s.st.Rig.Lon, Source: "mount"}
	default:
		s.st.Site = nil
	}
}

func (s *Session) resolveModelLocked() {
	if m, ok := models.ByKey(s.st.Settings.Model); ok {
		s.st.Model, s.st.ModelSource = &m, "selected"
		return
	}
	if s.st.Rig != nil && s.st.Rig.Detected != nil {
		m := *s.st.Rig.Detected
		s.st.Model, s.st.ModelSource = &m, "detected"
		return
	}
	if s.st.Rig != nil {
		if c := models.Fallbacks(nil, s.st.Rig.SensorW, s.st.Rig.SensorH); len(c) > 0 {
			m := c[0]
			s.st.Model, s.st.ModelSource = &m, "guessed from sensor size"
			return
		}
	}
	s.st.Model, s.st.ModelSource = nil, ""
}

// ---------------------------------------------------------------- connect

func (s *Session) Connect(ctx context.Context) error {
	s.mu.Lock()
	if s.st.Phase != "disconnected" && s.st.Phase != "ready" {
		s.mu.Unlock()
		return errors.New("busy - stop the alignment first")
	}
	s.st.Phase = "connecting"
	s.st.Warnings = nil
	set := s.st.Settings
	s.notifyLocked()
	s.mu.Unlock()

	if s.rig != nil {
		s.rig.Close()
		s.rig = nil
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	rig, err := connectAlpaca(cctx, set, s.info)
	cancel()
	if err != nil {
		s.update(func(st *State) { st.Phase = "disconnected"; st.Rig = nil })
		s.logf("error", "Connect failed: %v", err)
		return err
	}
	return s.attach(ctx, rig, &astapSolver{s: s})
}

// attach checks a freshly connected rig, warns about anything that will
// get in the way, and makes it the session's rig. Tests use it to plug in
// a simulated rig.
func (s *Session) attach(ctx context.Context, rig Rig, fs frameSolver) error {
	s.mu.Lock()
	set := s.st.Settings
	s.mu.Unlock()
	info := rig.Info()

	var warnings []string
	if info.AlignmentMode == 0 {
		warnings = append(warnings, "The Seestar reports Alt-Az mode. Polar alignment needs EQ mode on a wedge - switch it in the Seestar app.")
	}
	if info.Lat == nil && set.Lat == nil {
		warnings = append(warnings, "The mount didn't report its location. Enter latitude/longitude in the settings.")
	}
	if info.ClockOffsetS != nil && math.Abs(*info.ClockOffsetS) > 5 {
		if set.SyncClock {
			if err := rig.SyncClock(ctx); err != nil {
				warnings = append(warnings, fmt.Sprintf("Seestar clock is off by %.0f s and syncing it failed: %v", *info.ClockOffsetS, err))
			} else {
				s.info("Synced the Seestar clock (was off by %.0f s)", *info.ClockOffsetS)
			}
		} else {
			warnings = append(warnings, fmt.Sprintf("Seestar clock is off by %.0f s.", *info.ClockOffsetS))
		}
	}
	for _, w := range warnings {
		s.warn("%s", w)
	}

	s.mu.Lock()
	s.rig, s.solver = rig, fs
	s.st.Rig = &info
	s.st.Phase = "ready"
	s.st.Step = ""
	s.st.Warnings = warnings
	s.resolveSiteLocked()
	s.resolveModelLocked()
	s.notifyLocked()
	s.mu.Unlock()
	return nil
}

func (s *Session) Disconnect() {
	s.Stop()
	s.mu.Lock()
	if s.rig != nil {
		s.rig.Close()
	}
	s.rig, s.solver = nil, nil
	s.st.Phase, s.st.Step, s.st.Rig, s.st.Warnings = "disconnected", "", nil, nil
	s.resolveSiteLocked()
	s.resolveModelLocked()
	s.notifyLocked()
	s.mu.Unlock()
}

// ---------------------------------------------------------------- run

func (s *Session) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.st.Phase != "ready" {
		return fmt.Errorf("can't start while %s", s.st.Phase)
	}
	if s.st.Site == nil {
		return errors.New("site location unknown - enter latitude/longitude in the settings")
	}
	if s.st.Model == nil {
		return errors.New("select your Seestar model")
	}
	if _, ok := s.solver.(*astapSolver); ok && s.st.ASTAP.Path == "" {
		return errors.New("ASTAP not found - install astap_cli (see README)")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel, s.done = cancel, make(chan struct{})
	s.st.Phase = "measuring"
	s.st.Points, s.st.Initial, s.st.Error, s.st.History, s.st.Adjust = nil, nil, nil, nil, nil
	s.notifyLocked()
	go s.run(ctx, s.done)
	return nil
}

func (s *Session) Stop() {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
}

func (s *Session) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	defer func() {
		if r := recover(); r != nil {
			s.logf("error", "internal error: %v", r)
		}
		// Whatever happened, make sure the mount isn't left moving.
		bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = s.rig.MoveAxis(bg, 0)
		cancel()
		s.update(func(st *State) {
			if st.Phase == "measuring" || st.Phase == "adjusting" {
				st.Phase = "ready"
			}
			st.Step = ""
		})
		s.mu.Lock()
		s.cancel, s.done = nil, nil
		s.mu.Unlock()
	}()

	err := s.measureAndAdjust(ctx)
	switch {
	case err == nil || errors.Is(err, context.Canceled):
		s.info("Stopped")
	default:
		s.logf("error", "%v", err)
		s.update(func(st *State) { st.Warnings = append(st.Warnings, err.Error()) })
	}
}

func (s *Session) measureAndAdjust(ctx context.Context) error {
	s.mu.Lock()
	set, site, rig, info := s.st.Settings, *s.st.Site, s.rig, *s.st.Rig
	s.mu.Unlock()

	atm := astro.Atmosphere{}
	if set.Refraction {
		atm = astro.Atmosphere{Enabled: true, PressureHPa: set.PressureHPa, TempC: set.TempC}
	}
	s.info("Starting: site %.4f, %.4f; %g s exposures; %g° moves to the %s", site.Lat, site.Lon, set.ExposureS, set.MoveDeg, set.Direction)

	if set.Gain != nil {
		if err := rig.SetGain(ctx, *set.Gain); err != nil {
			s.warn("Couldn't set gain: %v", err)
		}
	}
	if err := rig.SetTracking(ctx, true); err != nil {
		s.warn("Couldn't switch tracking on: %v", err)
	}

	if set.StartEnabled {
		if err := s.slewToStart(ctx, rig, set, site); err != nil {
			return err
		}
	}

	var pts [3]astro.Vec
	var times [3]time.Time
	var off *hint // last (solved - mount) offset, refines solve hints
	for i := 0; i < 3; i++ {
		if i > 0 {
			s.step("Moving RA %g° (%d of 2)", set.MoveDeg, i)
			if err := s.moveRA(ctx, rig, info, set); err != nil {
				return err
			}
		}
		fr, sol, err := s.captureSolve(ctx, rig, set, fmt.Sprintf("Point %d of 3", i+1), off, true)
		if err != nil {
			return err
		}
		off = s.offset(ctx, rig, sol)
		pts[i] = astro.PointingVector(sol.RADeg, sol.DecDeg, fr.Mid(), site.Lat, site.Lon, atm)
		times[i] = fr.Mid()
		alt, az := astro.VecToAltAz(pts[i])
		s.update(func(st *State) {
			st.Points = append(st.Points, Point{N: i + 1, RADeg: sol.RADeg, DecDeg: sol.DecDeg,
				AltDeg: alt / astro.Deg, AzDeg: az / astro.Deg, Time: fr.Mid()})
		})
		s.info("Point %d: RA %.4f° Dec %.4f° (alt %.1f° az %.1f°)", i+1, sol.RADeg, sol.DecDeg, alt/astro.Deg, az/astro.Deg)
	}

	axis, err := polar.FitAxis(pts[0], pts[1], pts[2], site.Lat)
	if err != nil {
		return err
	}
	pole := polar.TargetPole(site.Lat, atm)
	e := polar.ComputeError(axis, site.Lat, pole)
	s.info("Polar error: altitude %+.1f′, azimuth %+.1f′ (total %.1f′)", e.AltDeg*60, e.AzDeg*60, e.TotalDeg*60)
	s.update(func(st *State) {
		st.Initial, st.Error = &e, &e
		st.History = []HistoryEntry{{T: times[2], Total: e.TotalDeg, Alt: e.AltDeg, Az: e.AzDeg}}
		st.Phase = "adjusting"
		st.Adjust = &AdjustInfo{}
	})

	tracker := polar.NewTracker(axis, pts[2], times[2], true)
	for n := 1; ctx.Err() == nil; n++ {
		fr, sol, err := s.captureSolve(ctx, rig, set, "Adjust the knobs", off, false)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.warn("Frame skipped: %v", err)
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
			}
			continue
		}
		p := astro.PointingVector(sol.RADeg, sol.DecDeg, fr.Mid(), site.Lat, site.Lon, atm)
		adj := tracker.Update(p, fr.Mid())
		e := polar.ComputeError(adj.Axis, site.Lat, pole)
		s.update(func(st *State) {
			st.Error = &e
			st.History = append(st.History, HistoryEntry{T: fr.Mid(), Total: e.TotalDeg, Alt: e.AltDeg, Az: e.AzDeg})
			if len(st.History) > 300 {
				st.History = st.History[len(st.History)-300:]
			}
			st.Adjust = &AdjustInfo{Frames: n, Conditioning: adj.Conditioning,
				AlphaDeg: adj.AlphaDeg, BetaDeg: adj.BetaDeg, Warnings: adj.Warnings}
		})
	}
	return ctx.Err()
}

// startTarget is the mount-frame (JNow) RA in hours and Dec in degrees of the
// configured starting position.
func startTarget(set Settings, site Site, now time.Time) (raHours, decDeg float64, desc string) {
	if set.StartMode == "radec" {
		return set.StartRAHours, set.StartDecDeg, fmt.Sprintf("RA %.3f h, Dec %+.2f°", set.StartRAHours, set.StartDecDeg)
	}
	v := astro.AltAzToVec(set.StartAltDeg*astro.Deg, set.StartAzDeg*astro.Deg)
	ra, dec := astro.HorizonToRADecNow(v, now, site.Lat, site.Lon)
	return ra, dec, fmt.Sprintf("alt %.1f° az %.1f° (RA %.3f h, Dec %+.2f°)", set.StartAltDeg, set.StartAzDeg, ra, dec)
}

// slewToStart is the first step of a run: slew to the saved starting
// position and wait for the mount to arrive and settle.
func (s *Session) slewToStart(ctx context.Context, rig Rig, set Settings, site Site) error {
	ra, dec, desc := startTarget(set, site, time.Now())
	s.step("Slewing to the starting position")
	s.info("Slewing to the starting position: %s", desc)
	if err := rig.SetTracking(ctx, true); err != nil { // slews need tracking on
		s.warn("Couldn't switch tracking on: %v", err)
	}
	if err := rig.SlewTo(ctx, ra, dec); err != nil {
		return fmt.Errorf("slewing to the starting position: %w", err)
	}
	abort := func() {
		bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = rig.AbortSlew(bg)
	}
	t0 := time.Now()
	for {
		select {
		case <-ctx.Done():
			abort()
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
		// A mount may not report "slewing" until a moment after the command.
		if sl, err := rig.Slewing(ctx); err == nil && !sl && time.Since(t0) > 1500*time.Millisecond {
			break
		}
		if time.Since(t0) > 3*time.Minute {
			abort()
			return errors.New("the slew to the starting position didn't finish within 3 minutes")
		}
	}
	s.step("Settling")
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(set.SettleS * float64(time.Second))):
	}
	if gotRA, gotDec, err := rig.Position(ctx); err == nil {
		d := math.Hypot(raDistDeg(gotRA, ra)*math.Cos(dec*astro.Deg), gotDec-dec)
		if d > 1 {
			s.warn("The mount stopped %.1f° from the starting position (RA %.3f h, Dec %+.2f°)", d, gotRA, gotDec)
		} else {
			s.info("At the starting position (RA %.3f h, Dec %+.2f°)", gotRA, gotDec)
		}
	}
	return nil
}

func (s *Session) offset(ctx context.Context, rig Rig, sol solver.Solution) *hint {
	ra, dec, err := rig.Position(ctx)
	if err != nil {
		return nil
	}
	return &hint{RA: sol.RADeg - ra*15, Dec: sol.DecDeg - dec}
}

func (s *Session) captureSolve(ctx context.Context, rig Rig, set Settings, label string, off *hint, wide bool) (Frame, solver.Solution, error) {
	s.step("%s: exposing %g s", label, set.ExposureS)
	fr, err := rig.Expose(ctx, set.ExposureS)
	if err != nil {
		return Frame{}, solver.Solution{}, err
	}
	binned := imaging.Bin2x2(fr.Image)
	jpg, jerr := imaging.PreviewJPEG(binned, 1000)
	s.mu.Lock()
	if jerr == nil {
		s.jpeg = jpg
	}
	id := 1
	if s.st.Frame != nil {
		id = s.st.Frame.ID + 1
	}
	s.st.Frame = &FrameInfo{ID: id, Time: fr.Mid(), Solving: true}
	s.notifyLocked()
	s.mu.Unlock()

	var h *hint
	if ra, dec, err := rig.Position(ctx); err == nil {
		h = &hint{RA: ra * 15, Dec: dec}
		if off != nil {
			h.RA += off.RA
			h.Dec = math.Max(-90, math.Min(90, h.Dec+off.Dec))
		}
	}
	s.step("%s: plate solving", label)
	sol, usedModel, err := s.solver.Solve(ctx, fr, binned, h)
	if err != nil && wide && ctx.Err() == nil && h != nil && !errors.Is(err, errFatalSolve) {
		s.warn("Solve failed near the mount position (%v); widening the search", err)
		sol, usedModel, err = s.solver.Solve(ctx, fr, binned, &hint{RA: h.RA, Dec: h.Dec, Wide: true})
	}
	s.update(func(st *State) {
		if st.Frame == nil || st.Frame.ID != id {
			return
		}
		st.Frame.Solving = false
		if err != nil {
			st.Frame.Error = err.Error()
			return
		}
		st.Frame.Solved, st.Frame.RADeg, st.Frame.DecDeg = true, sol.RADeg, sol.DecDeg
		st.Frame.ScaleArcsec, st.Frame.SolveS = sol.ScaleArcsec, sol.Seconds
		if usedModel != "" {
			if m, ok := models.ByKey(usedModel); ok && (st.Model == nil || st.Model.Key != m.Key) {
				st.Model, st.ModelSource = &m, "corrected by plate solve"
			}
		}
	})
	if usedModel != "" {
		s.info("Plate solve matched the %s scale; using that model", usedModel)
	}
	return fr, sol, err
}

func chooseRate(want float64, rates []Rate) float64 {
	if len(rates) == 0 {
		return want
	}
	best := 0.0
	for _, r := range rates {
		if want >= r.Min && want <= r.Max {
			return want
		}
		if r.Max < want && r.Max > best {
			best = r.Max
		}
	}
	if best == 0 {
		// everything is faster than requested: take the slowest available
		best = rates[0].Min
		for _, r := range rates {
			best = math.Min(best, r.Min)
		}
		if best <= 0 {
			best = want
		}
	}
	return best
}

func raDistDeg(a, b float64) float64 {
	d := math.Abs(a-b) * 15
	d = math.Mod(d, 360)
	return 180 - math.Abs(d-180)
}

func (s *Session) moveRA(ctx context.Context, rig Rig, info RigInfo, set Settings) (err error) {
	rate := chooseRate(set.MoveRate, info.AxisRates)
	if set.Direction == "west" {
		rate = -rate
	}
	ra0, dec0, err := rig.Position(ctx)
	if err != nil {
		return fmt.Errorf("reading mount position: %w", err)
	}
	if err := rig.MoveAxis(ctx, rate); err != nil {
		return fmt.Errorf("MoveAxis: %w", err)
	}
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if e := rig.MoveAxis(bg, 0); e != nil && err == nil {
			err = fmt.Errorf("stopping the mount: %w", e)
		}
	}
	defer stop()

	t0 := time.Now()
	limit := time.Duration((set.MoveDeg/math.Abs(rate)*2 + 15) * float64(time.Second))
	var dist float64
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
		ra, dec, perr := rig.Position(ctx)
		if perr != nil {
			continue
		}
		dist = raDistDeg(ra, ra0)
		if math.Abs(dec-dec0) > 1 {
			return fmt.Errorf("Dec changed by %.1f° while moving RA - the mount isn't moving along RA alone. Is the Seestar in EQ mode?", dec-dec0)
		}
		if dist >= set.MoveDeg {
			break
		}
		if time.Since(t0) > 6*time.Second && dist < 0.2 {
			return errors.New("the mount doesn't move with MoveAxis (RA unchanged after 6 s)")
		}
		if time.Since(t0) > limit {
			s.warn("Move timed out after %.1f° of %g°", dist, set.MoveDeg)
			break
		}
	}
	stop()
	if err != nil {
		return err
	}
	for i := 0; i < 75; i++ { // up to 15 s
		if sl, e := rig.Slewing(ctx); e != nil || !sl {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	s.step("Settling")
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(set.SettleS * float64(time.Second))):
	}
	if e := rig.SetTracking(ctx, true); e != nil {
		s.warn("Couldn't re-enable tracking: %v", e)
	}
	s.info("Moved RA %.1f°", dist)
	return nil
}

// ---------------------------------------------------------------- ASTAP

var errFatalSolve = errors.New("fatal solve error")

type astapSolver struct{ s *Session }

func (a *astapSolver) Solve(ctx context.Context, f Frame, binned imaging.Image, h *hint) (solver.Solution, string, error) {
	a.s.mu.Lock()
	set, model, info, astap := a.s.st.Settings, a.s.st.Model, a.s.st.Rig, a.s.st.ASTAP.Path
	a.s.mu.Unlock()

	radius := set.SearchRadiusDeg
	var ra, dec *float64
	if h != nil {
		r, d := h.RA, h.Dec
		if h.Wide {
			radius = 60
		}
		ra, dec = &r, &d
	}
	pixel := info.PixelUM
	path := filepath.Join(a.s.workDir, "frame.fits")
	if err := os.MkdirAll(a.s.workDir, 0o755); err != nil {
		return solver.Solution{}, "", err
	}
	if err := imaging.WriteFITS(path, binned, imaging.FITSMeta{Exposure: f.Exposure, DateObs: f.Start,
		PixelUM: pixelOr(pixel, model) * 2, FocalMM: model.FocalMM}); err != nil {
		return solver.Solution{}, "", fmt.Errorf("writing FITS: %w", err)
	}
	var lastErr error
	for i, m := range models.Fallbacks(model, binned.W*2, binned.H*2) {
		if i > 0 && (m.Width*m.Height != model.Width*model.Height) {
			break // only retry models with the same sensor
		}
		sol, err := solver.Solve(ctx, path, solver.Options{
			Binary: astap, FOVDeg: m.FOVDeg(binned.H, pixel, 2), RADeg: ra, DecDeg: dec, RadiusDeg: radius,
			Database: set.ASTAPDatabase, DatabaseDir: set.ASTAPDatabaseDir,
		})
		if err == nil {
			used := ""
			if i > 0 {
				used = m.Key
			}
			return sol, used, nil
		}
		lastErr = err
		if !errors.Is(err, solver.ErrNoSolution) {
			return solver.Solution{}, "", fmt.Errorf("%w: %w", errFatalSolve, err)
		}
	}
	return solver.Solution{}, "", lastErr
}

func pixelOr(px float64, m *models.Model) float64 {
	if px > 0 {
		return px
	}
	return m.PixelUM
}
