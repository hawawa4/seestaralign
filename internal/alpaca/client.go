// Package alpaca is a small ASCOM Alpaca client - just what polar alignment needs.
package alpaca

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// SeestarNativePort is ZWO's on-device Alpaca server.
const SeestarNativePort = 32323

type Error struct {
	Number  int
	Message string
}

func (e *Error) Error() string {
	if e.Number != 0 {
		return fmt.Sprintf("%s (Alpaca error 0x%X)", e.Message, e.Number)
	}
	return e.Message
}

type Client struct {
	Base     string
	http     *http.Client
	clientID int
	tid      atomic.Uint32
}

func NewClient(base string) *Client {
	return &Client{
		Base:     strings.TrimRight(base, "/"),
		http:     &http.Client{Timeout: 20 * time.Second},
		clientID: rand.IntN(65535) + 1,
	}
}

func (c *Client) ids(v url.Values) url.Values {
	if v == nil {
		v = url.Values{}
	}
	v.Set("ClientID", strconv.Itoa(c.clientID))
	v.Set("ClientTransactionID", strconv.FormatUint(uint64(c.tid.Add(1)), 10))
	return v
}

type response struct {
	Value        json.RawMessage
	ErrorNumber  int
	ErrorMessage string
}

func decode(resp *http.Response) (json.RawMessage, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, &Error{Message: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, msg)}
	}
	var r response
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("bad Alpaca response: %w", err)
	}
	if r.ErrorNumber != 0 {
		msg := r.ErrorMessage
		if msg == "" {
			msg = "device error"
		}
		return nil, &Error{Number: r.ErrorNumber, Message: msg}
	}
	return r.Value, nil
}

func (c *Client) Get(ctx context.Context, dev string, num int, prop string, params url.Values) (json.RawMessage, error) {
	u := fmt.Sprintf("%s/api/v1/%s/%d/%s?%s", c.Base, dev, num, strings.ToLower(prop), c.ids(params).Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	return decode(resp)
}

func (c *Client) Put(ctx context.Context, dev string, num int, method string, form url.Values) (json.RawMessage, error) {
	u := fmt.Sprintf("%s/api/v1/%s/%d/%s", c.Base, dev, num, strings.ToLower(method))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, strings.NewReader(c.ids(form).Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	return decode(resp)
}

// typed helpers

func (c *Client) Float(ctx context.Context, dev string, num int, prop string) (float64, error) {
	raw, err := c.Get(ctx, dev, num, prop, nil)
	if err != nil {
		return 0, err
	}
	var v float64
	return v, json.Unmarshal(raw, &v)
}

func (c *Client) Int(ctx context.Context, dev string, num int, prop string) (int, error) {
	f, err := c.Float(ctx, dev, num, prop)
	return int(f), err
}

func (c *Client) Bool(ctx context.Context, dev string, num int, prop string) (bool, error) {
	raw, err := c.Get(ctx, dev, num, prop, nil)
	if err != nil {
		return false, err
	}
	var v bool
	return v, json.Unmarshal(raw, &v)
}

func (c *Client) String(ctx context.Context, dev string, num int, prop string) (string, error) {
	raw, err := c.Get(ctx, dev, num, prop, nil)
	if err != nil {
		return "", err
	}
	var v string
	return v, json.Unmarshal(raw, &v)
}

func FormBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func FormFloat(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

type Device struct {
	Type     string `json:"type"`
	Number   int    `json:"number"`
	Name     string `json:"name"`
	UniqueID string `json:"unique_id"`
}

func (c *Client) ConfiguredDevices(ctx context.Context) ([]Device, error) {
	u := fmt.Sprintf("%s/management/v1/configureddevices?%s", c.Base, c.ids(nil).Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	raw, err := decode(resp)
	if err != nil {
		return nil, err
	}
	var list []struct {
		DeviceName, DeviceType, UniqueID string
		DeviceNumber                     int
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	out := make([]Device, 0, len(list))
	for _, d := range list {
		out = append(out, Device{Type: strings.ToLower(d.DeviceType), Number: d.DeviceNumber, Name: d.DeviceName, UniqueID: d.UniqueID})
	}
	return out, nil
}

// ImageArray downloads the camera image, preferring the binary ImageBytes
// transfer. Returns data in Alpaca [x][y] order (y fastest) plus dimensions.
func (c *Client) ImageArray(ctx context.Context, num int) (data []float32, dimX, dimY int, err error) {
	u := fmt.Sprintf("%s/api/v1/camera/%d/imagearray?%s", c.Base, num, c.ids(nil).Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, 0, err
	}
	req.Header.Set("Accept", "application/imagebytes")
	// big images over Wi-Fi are slow; don't use the default client timeout
	resp, err := (&http.Client{Timeout: 3 * time.Minute}).Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/imagebytes") {
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, 0, 0, err
		}
		return ParseImageBytes(body)
	}
	raw, err := decode(resp)
	if err != nil {
		return nil, 0, 0, err
	}
	return parseJSONImage(raw)
}

// ParseImageBytes decodes an Alpaca ImageBytes (ArrayMetadataV1) payload.
func ParseImageBytes(b []byte) ([]float32, int, int, error) {
	if len(b) < 44 {
		return nil, 0, 0, fmt.Errorf("short ImageBytes payload")
	}
	le := binary.LittleEndian
	i32 := func(off int) int { return int(int32(le.Uint32(b[off:]))) }
	version, errNum, start := i32(0), i32(4), i32(16)
	txType, rank, d1, d2, d3 := i32(24), i32(28), i32(32), i32(36), i32(40)
	if version != 1 {
		return nil, 0, 0, fmt.Errorf("unsupported ImageBytes metadata version %d", version)
	}
	if errNum != 0 {
		return nil, 0, 0, &Error{Number: errNum, Message: string(b[start:])}
	}
	planes := 1
	if rank == 3 {
		planes = d3
	}
	n := d1 * d2 * planes
	sizes := map[int]int{1: 2, 2: 4, 3: 8, 4: 4, 5: 8, 6: 1, 7: 8, 8: 2, 9: 4}
	sz, ok := sizes[txType]
	if !ok {
		return nil, 0, 0, fmt.Errorf("unsupported ImageBytes element type %d", txType)
	}
	p := b[start:]
	if len(p) < n*sz {
		return nil, 0, 0, fmt.Errorf("ImageBytes payload truncated")
	}
	out := make([]float32, d1*d2)
	for i := 0; i < n; i++ {
		var v float64
		o := p[i*sz:]
		switch txType {
		case 1:
			v = float64(int16(le.Uint16(o)))
		case 2:
			v = float64(int32(le.Uint32(o)))
		case 3:
			v = math.Float64frombits(le.Uint64(o))
		case 4:
			v = float64(math.Float32frombits(le.Uint32(o)))
		case 5:
			v = float64(le.Uint64(o))
		case 6:
			v = float64(o[0])
		case 7:
			v = float64(int64(le.Uint64(o)))
		case 8:
			v = float64(le.Uint16(o))
		case 9:
			v = float64(le.Uint32(o))
		}
		// rank 3 is [x][y][plane]: average the planes into mono
		out[i/planes] += float32(v / float64(planes))
	}
	return out, d1, d2, nil
}

func parseJSONImage(raw json.RawMessage) ([]float32, int, int, error) {
	var a2 [][]float64
	if err := json.Unmarshal(raw, &a2); err == nil && len(a2) > 0 {
		dx, dy := len(a2), len(a2[0])
		out := make([]float32, dx*dy)
		for x, col := range a2 {
			for y := 0; y < dy && y < len(col); y++ {
				out[x*dy+y] = float32(col[y])
			}
		}
		return out, dx, dy, nil
	}
	var a3 [][][]float64
	if err := json.Unmarshal(raw, &a3); err != nil || len(a3) == 0 || len(a3[0]) == 0 {
		return nil, 0, 0, fmt.Errorf("unrecognised ImageArray JSON")
	}
	dx, dy := len(a3), len(a3[0])
	out := make([]float32, dx*dy)
	for x := range a3 {
		for y := range a3[x] {
			var s float64
			for _, v := range a3[x][y] {
				s += v
			}
			out[x*dy+y] = float32(s / float64(len(a3[x][y])))
		}
	}
	return out, dx, dy, nil
}
