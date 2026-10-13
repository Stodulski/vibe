package social

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"time"

	"github.com/stodulski/vibe-server/internal/storage"
)

// ProbeImageKey is where the 3:4 probe image lives. It is a fixed key because
// its bytes never change, so a re-upload at each boot replaces the same object.
const ProbeImageKey = "social/probe/3x4.jpg"

// ProbeImage returns the plain 1080x1440 JPEG the 3:4 probe asks Instagram to
// accept. It is one flat colour, so its bytes are the same on every encode.
func ProbeImage() ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 1080, 1440))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 0x1f, G: 0x6f, B: 0x8b, A: 0xff}}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("social: encode probe image: %w", err)
	}
	return buf.Bytes(), nil
}

// EnsureProbeImage uploads the probe image and returns the public URL Meta
// fetches it from. The upload runs at each boot: the bytes never change, so the
// object is simply written again.
func EnsureProbeImage(ctx context.Context, media MediaStorage) (string, error) {
	body, err := ProbeImage()
	if err != nil {
		return "", err
	}
	if err := media.PutObject(ctx, storage.Object{Key: ProbeImageKey, ContentType: "image/jpeg", Body: body}); err != nil {
		return "", fmt.Errorf("social: upload probe image: %w", err)
	}
	url := media.PublicURL(ProbeImageKey)
	if url == "" {
		return "", errors.New("social: the public media URL is not configured, so the probe cannot run")
	}
	return url, nil
}

// threadsCheck is the Threads line of the connection email.
type threadsCheck struct {
	store      ThreadsStore
	configured bool
	connectURL string
	now        func() time.Time
}

// NewThreadsCheck returns the Threads line of the connection email.
func NewThreadsCheck(store ThreadsStore, configured bool, connectURL string) Check {
	return threadsCheck{store: store, configured: configured, connectURL: connectURL, now: time.Now}
}

// Check reports whether the app is configured and whether the account is
// connected, and how long its token lasts. The token itself is never shown.
func (t threadsCheck) Check(ctx context.Context, _ ConnectionReport) ReportLine {
	if !t.configured {
		return ReportLine{Text: "Threads: la app no está configurada (THREADS_APP_ID y THREADS_APP_SECRET)."}
	}
	conn, ok, err := t.store.LoadThreads(ctx)
	switch {
	case err != nil:
		return ReportLine{Text: "Threads: no se pudo leer la conexión.", Problem: true}
	case !ok:
		return ReportLine{Text: "Threads: no conectado. Conectar: " + t.connectURL + "."}
	}
	now := t.now()
	if !now.Before(conn.ExpiresAt) {
		return ReportLine{Text: "Threads: el token de @" + conn.Username + " venció. Reconectar: " + t.connectURL + ".", Problem: true}
	}
	days := int(conn.ExpiresAt.Sub(now).Hours() / 24)
	return ReportLine{Text: fmt.Sprintf("Threads: @%s conectado, el token vence en %d días.", conn.Username, days)}
}
