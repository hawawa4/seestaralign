package session

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"seestaralign/internal/alpaca"
	"seestaralign/internal/astro"
	"seestaralign/internal/imaging"
	"seestaralign/internal/models"
)

// Frame is one exposure.
type Frame struct {
	Image    imaging.Image // row-major mono, sensor resolution
	Start    time.Time
	Exposure float64
	Truth    *astro.Vec // simulator only: physical pointing at mid-exposure
}

func (f Frame) Mid() time.Time {
	return f.Start.Add(time.Duration(f.Exposure / 2 * float64(time.Second)))
}

type Rate struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// RigInfo is what we learned about the hardware on connect.
type RigInfo struct {
	BaseURL        string          `json:"base_url"`
	Camera         alpaca.Device   `json:"camera"`
	Telescope      alpaca.Device   `json:"telescope"`
	Cameras        []alpaca.Device `json:"cameras"`
	Telescopes     []alpaca.Device `json:"telescopes"`
	CameraDesc     string          `json:"camera_description"`
	SensorW        int             `json:"sensor_w"`
	SensorH        int             `json:"sensor_h"`
	PixelUM        float64         `json:"pixel_um"`
	Lat            *float64        `json:"lat"`
	Lon            *float64        `json:"lon"`
	AlignmentMode  int             `json:"alignment_mode"` // 0 alt-az, 1 polar, 2 german polar, -1 unknown
	CanSetTracking bool            `json:"can_set_tracking"`
	AxisRates      []Rate          `json:"axis_rates"`
	ClockOffsetS   *float64        `json:"clock_offset_s"`
	Detected       *models.Model   `json:"detected_model"`
	Simulated      bool            `json:"simulated"`
}

// Rig is the telescope + camera pair.
type Rig interface {
	Info() RigInfo
	Position(ctx context.Context) (raHours, decDeg float64, err error)
	SetTracking(ctx context.Context, on bool) error
	MoveAxis(ctx context.Context, rateDegPerSec float64) error // primary (RA) axis; 0 stops
	SlewTo(ctx context.Context, raHours, decDeg float64) error // starts the slew and returns; poll Slewing
	AbortSlew(ctx context.Context) error
	Slewing(ctx context.Context) (bool, error)
	SetGain(ctx context.Context, gain int) error
	SyncClock(ctx context.Context) error
	Expose(ctx context.Context, seconds float64) (Frame, error)
	Close()
}

// ---------------------------------------------------------------- Alpaca

type alpacaRig struct {
	c    *alpaca.Client
	info RigInfo
	cam  int
	tel  int
}

func pickDevice(list []alpaca.Device, want int, avoid ...string) (alpaca.Device, bool) {
	if len(list) == 0 {
		return alpaca.Device{}, false
	}
	if want >= 0 {
		for _, d := range list {
			if d.Number == want {
				return d, true
			}
		}
	}
	for _, d := range list {
		n := strings.ToLower(d.Name)
		bad := false
		for _, a := range avoid {
			bad = bad || strings.Contains(n, a)
		}
		if !bad {
			return d, true
		}
	}
	return list[0], true
}

// connectAlpaca finds the device, connects camera + telescope and reads what
// we need. logf receives progress messages.
func connectAlpaca(ctx context.Context, set Settings, logf func(string, ...any)) (*alpacaRig, error) {
	host := strings.TrimSpace(set.Host)
	if host == "" {
		host = "seestar.local"
	}
	// allow "host:port" in the host field
	if h, p, ok := strings.Cut(host, ":"); ok && !strings.Contains(p, ":") {
		host = h
		fmt.Sscan(p, &set.Port)
	}
	ip, err := alpaca.ResolveHost(ctx, host)
	if err != nil {
		return nil, err
	}
	if ip != host {
		logf("%s resolved to %s", host, ip)
	}
	port := set.Port
	if port == 0 {
		if found, _ := alpaca.Discover(ip, 1200*time.Millisecond); len(found) > 0 {
			port = found[0].Port
			logf("Alpaca discovery: port %d", port)
		} else {
			port = alpaca.SeestarNativePort
			logf("No discovery reply, trying the Seestar default port %d", port)
		}
	}
	c := alpaca.NewClient(fmt.Sprintf("http://%s:%d", ip, port))
	r := &alpacaRig{c: c}
	r.info.BaseURL = c.Base
	r.info.AlignmentMode = -1

	devs, err := c.ConfiguredDevices(ctx)
	if err != nil {
		return nil, fmt.Errorf("talking to Alpaca at %s: %w (is Alpaca enabled in the Seestar app?)", c.Base, err)
	}
	for _, d := range devs {
		switch d.Type {
		case "camera":
			r.info.Cameras = append(r.info.Cameras, d)
		case "telescope":
			r.info.Telescopes = append(r.info.Telescopes, d)
		}
	}
	cam, ok := pickDevice(r.info.Cameras, set.CameraNum, "wide", "guide")
	if !ok {
		return nil, errors.New("no Camera device found on the Alpaca server")
	}
	tel, ok := pickDevice(r.info.Telescopes, set.TelescopeNum)
	if !ok {
		return nil, errors.New("no Telescope device found on the Alpaca server")
	}
	r.info.Camera, r.info.Telescope, r.cam, r.tel = cam, tel, cam.Number, tel.Number
	logf("Using camera #%d %q and telescope #%d %q", cam.Number, cam.Name, tel.Number, tel.Name)

	for _, d := range []struct {
		typ string
		n   int
	}{{"camera", r.cam}, {"telescope", r.tel}} {
		if _, err := c.Put(ctx, d.typ, d.n, "connected", url.Values{"Connected": {"True"}}); err != nil {
			return nil, fmt.Errorf("connecting %s: %w", d.typ, err)
		}
		for i := 0; i < 40; i++ {
			if ok, err := c.Bool(ctx, d.typ, d.n, "connected"); err == nil && ok {
				break
			}
			time.Sleep(250 * time.Millisecond)
		}
	}

	r.info.CameraDesc, _ = c.String(ctx, "camera", r.cam, "description")
	camName, _ := c.String(ctx, "camera", r.cam, "name")
	telName, _ := c.String(ctx, "telescope", r.tel, "name")
	telDesc, _ := c.String(ctx, "telescope", r.tel, "description")
	if m, ok := models.Detect(cam.Name, camName, r.info.CameraDesc, tel.Name, telName, telDesc); ok {
		r.info.Detected = &m
	}
	r.info.SensorW, _ = c.Int(ctx, "camera", r.cam, "cameraxsize")
	r.info.SensorH, _ = c.Int(ctx, "camera", r.cam, "cameraysize")
	if px, err := c.Float(ctx, "camera", r.cam, "pixelsizey"); err == nil && px >= 1 && px <= 20 {
		r.info.PixelUM = px
	}
	if lat, err := c.Float(ctx, "telescope", r.tel, "sitelatitude"); err == nil {
		lon, err2 := c.Float(ctx, "telescope", r.tel, "sitelongitude")
		if err2 == nil && !(lat == 0 && lon == 0) {
			r.info.Lat, r.info.Lon = &lat, &lon
		}
	}
	if m, err := c.Int(ctx, "telescope", r.tel, "alignmentmode"); err == nil {
		r.info.AlignmentMode = m
	}
	r.info.CanSetTracking, _ = c.Bool(ctx, "telescope", r.tel, "cansettracking")
	if raw, err := c.Get(ctx, "telescope", r.tel, "axisrates", url.Values{"Axis": {"0"}}); err == nil {
		var rates []struct{ Minimum, Maximum float64 }
		if jsonUnmarshal(raw, &rates) == nil {
			for _, x := range rates {
				r.info.AxisRates = append(r.info.AxisRates, Rate{x.Minimum, x.Maximum})
			}
		}
	}
	if s, err := c.String(ctx, "telescope", r.tel, "utcdate"); err == nil {
		if t, err := parseUTC(s); err == nil {
			off := t.Sub(time.Now()).Seconds()
			r.info.ClockOffsetS = &off
		}
	}
	return r, nil
}

func parseUTC(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.9999999", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("bad UTCDate %q", s)
}

func (r *alpacaRig) Info() RigInfo { return r.info }
func (r *alpacaRig) Close()        {}

func (r *alpacaRig) Position(ctx context.Context) (float64, float64, error) {
	ra, err := r.c.Float(ctx, "telescope", r.tel, "rightascension")
	if err != nil {
		return 0, 0, err
	}
	dec, err := r.c.Float(ctx, "telescope", r.tel, "declination")
	return ra, dec, err
}

func (r *alpacaRig) SetTracking(ctx context.Context, on bool) error {
	_, err := r.c.Put(ctx, "telescope", r.tel, "tracking", url.Values{"Tracking": {alpaca.FormBool(on)}})
	return err
}

func (r *alpacaRig) MoveAxis(ctx context.Context, rate float64) error {
	_, err := r.c.Put(ctx, "telescope", r.tel, "moveaxis", url.Values{"Axis": {"0"}, "Rate": {alpaca.FormFloat(rate)}})
	return err
}

func (r *alpacaRig) SlewTo(ctx context.Context, raHours, decDeg float64) error {
	_, err := r.c.Put(ctx, "telescope", r.tel, "slewtocoordinatesasync",
		url.Values{"RightAscension": {alpaca.FormFloat(raHours)}, "Declination": {alpaca.FormFloat(decDeg)}})
	return err
}

func (r *alpacaRig) AbortSlew(ctx context.Context) error {
	_, err := r.c.Put(ctx, "telescope", r.tel, "abortslew", nil)
	return err
}

func (r *alpacaRig) Slewing(ctx context.Context) (bool, error) {
	return r.c.Bool(ctx, "telescope", r.tel, "slewing")
}

func (r *alpacaRig) SetGain(ctx context.Context, gain int) error {
	_, err := r.c.Put(ctx, "camera", r.cam, "gain", url.Values{"Gain": {fmt.Sprint(gain)}})
	return err
}

func (r *alpacaRig) SyncClock(ctx context.Context) error {
	_, err := r.c.Put(ctx, "telescope", r.tel, "utcdate",
		url.Values{"UTCDate": {time.Now().UTC().Format("2006-01-02T15:04:05.000Z")}})
	return err
}

func (r *alpacaRig) Expose(ctx context.Context, seconds float64) (Frame, error) {
	start := time.Now()
	if _, err := r.c.Put(ctx, "camera", r.cam, "startexposure",
		url.Values{"Duration": {alpaca.FormFloat(seconds)}, "Light": {"True"}}); err != nil {
		return Frame{}, fmt.Errorf("starting exposure: %w", err)
	}
	deadline := start.Add(time.Duration(seconds*float64(time.Second)) + 2*time.Minute)
	for {
		select {
		case <-ctx.Done():
			bg, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_, _ = r.c.Put(bg, "camera", r.cam, "abortexposure", nil)
			cancel()
			return Frame{}, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
		ready, err := r.c.Bool(ctx, "camera", r.cam, "imageready")
		if err == nil && ready {
			break
		}
		if time.Now().After(deadline) {
			return Frame{}, errors.New("timed out waiting for the image")
		}
	}
	data, dx, dy, err := r.c.ImageArray(ctx, r.cam)
	if err != nil {
		return Frame{}, fmt.Errorf("downloading image: %w", err)
	}
	return Frame{Image: imaging.FromAlpaca(data, dx, dy), Start: start, Exposure: seconds}, nil
}
