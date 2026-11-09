package imaging

import (
	"bytes"
	"encoding/binary"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFromAlpacaAndBin(t *testing.T) {
	// 4 wide (x) by 2 tall (y), Alpaca order is [x][y]
	data := []float32{0, 10, 1, 11, 2, 12, 3, 13}
	img := FromAlpaca(data, 4, 2)
	if img.W != 4 || img.H != 2 || img.Pix[0*4+3] != 3 || img.Pix[1*4+0] != 10 {
		t.Fatalf("bad transpose: %+v", img)
	}
	b := Bin2x2(img)
	if b.W != 2 || b.H != 1 || b.Pix[0] != (0+1+10+11)/4.0 {
		t.Fatalf("bad bin: %+v", b)
	}
}

func TestWriteFITS(t *testing.T) {
	img := Image{W: 3, H: 2, Pix: []float32{0, 1, 65535, 100, 200, 300}}
	p := filepath.Join(t.TempDir(), "x.fits")
	if err := WriteFITS(p, img, FITSMeta{Exposure: 2}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if len(b)%2880 != 0 || !strings.HasPrefix(string(b), "SIMPLE  =                    T") {
		t.Fatal("bad FITS framing")
	}
	hdr := string(b[:2880])
	for _, want := range []string{"NAXIS1  =                    3", "BZERO   =                32768", "EXPTIME =                    2", "END"} {
		if !strings.Contains(hdr, want) {
			t.Errorf("missing %q", want)
		}
	}
	// first stored row is the bottom image row (y=1): 100 -> int16 100-32768
	v := int16(binary.BigEndian.Uint16(b[2880:]))
	if int(v)+32768 != 100 {
		t.Errorf("first pixel %d", int(v)+32768)
	}
}

func TestPreviewJPEG(t *testing.T) {
	img := Image{W: 400, H: 300, Pix: make([]float32, 400*300)}
	for i := range img.Pix {
		img.Pix[i] = 1000 + float32(i%7)
	}
	img.Pix[150*400+200] = 60000
	b, err := PreviewJPEG(img, 200)
	if err != nil {
		t.Fatal(err)
	}
	im, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil || im.Bounds().Dx() != 200 {
		t.Fatalf("bad jpeg: %v %v", err, im.Bounds())
	}
}
