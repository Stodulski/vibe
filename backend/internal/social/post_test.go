package social

import (
	"errors"
	"strings"
	"testing"
)

func validFields() Fields {
	return Fields{
		PublishAt: "2026-10-20T19:00:00-03:00",
		Networks:  []string{"instagram", "facebook"},
		Type:      "image",
		Caption:   "Caja de turnos",
		AltText:   "Una caja con turnos",
		Media:     []string{"1.jpg"},
	}
}

func TestValidatePostAcceptsWellFormedPosts(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		id     string
		mutate func(*Fields)
		kind   Kind
		media  int
	}{
		"single image": {id: "2026-10-20-caja", kind: TypeImage, media: 1},
		"carousel of two": {id: "carrusel-2", kind: TypeCarousel, media: 2, mutate: func(f *Fields) {
			f.Type = "carousel"
			f.Media = []string{"1.jpg", "2.jpeg"}
		}},
		"reel draft": {id: "reel-1", kind: TypeReel, media: 1, mutate: func(f *Fields) {
			f.Type = "reel"
			f.Media = []string{"clip.mp4"}
			f.Draft = true
		}},
		"story image without caption": {id: "historia-1", kind: TypeStory, media: 1, mutate: func(f *Fields) {
			f.Type = "story"
			f.Caption = ""
			f.Media = []string{"1.jpg"}
		}},
		"story video": {id: "historia-2", kind: TypeStory, media: 1, mutate: func(f *Fields) {
			f.Type = "story"
			f.Networks = []string{"instagram"}
			f.Media = []string{"1.mp4"}
		}},
		"story with a caption the story does not send": {id: "historia-3", kind: TypeStory, media: 1, mutate: func(f *Fields) {
			f.Type = "story"
			f.Networks = []string{"facebook"}
			f.Media = []string{"1.jpeg"}
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := validFields()
			if tc.mutate != nil {
				tc.mutate(&f)
			}
			post, errs := ValidatePost(tc.id, f)
			if errs != nil {
				t.Fatalf("ValidatePost() errors = %v, want none", errs)
			}
			if post.Kind != tc.kind || len(post.Media) != tc.media {
				t.Fatalf("post kind=%q media=%d, want %q and %d", post.Kind, len(post.Media), tc.kind, tc.media)
			}
			if post.Status != StatusScheduled {
				t.Fatalf("status = %q, want scheduled", post.Status)
			}
		})
	}
}

func TestValidatePostRejectsMalformedPosts(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		id     string
		mutate func(*Fields)
		field  string
	}{
		"bad id":            {id: "Caja_Turnos", field: "id"},
		"id too long":       {id: strings.Repeat("a", 82), field: "id"},
		"time without zone": {id: "p", mutate: func(f *Fields) { f.PublishAt = "2026-10-20T19:00:00" }, field: "publish_at"},
		"no networks":       {id: "p", mutate: func(f *Fields) { f.Networks = nil }, field: "networks"},
		"duplicate network": {id: "p", mutate: func(f *Fields) { f.Networks = []string{"facebook", "facebook"} }, field: "networks"},
		"unknown network":   {id: "p", mutate: func(f *Fields) { f.Networks = []string{"tiktok"} }, field: "networks"},
		"unknown type":      {id: "p", mutate: func(f *Fields) { f.Type = "live" }, field: "type"},
		"story to threads": {id: "p", mutate: func(f *Fields) {
			f.Type = "story"
			f.Networks = []string{"instagram", "threads"}
			f.Media = []string{"1.jpg"}
		}, field: "networks"},
		"story to threads alone": {id: "p", mutate: func(f *Fields) {
			f.Type = "story"
			f.Networks = []string{"threads"}
			f.Media = []string{"1.jpg"}
		}, field: "networks"},
		"story with threads text": {id: "p", mutate: func(f *Fields) {
			f.Type = "story"
			f.Networks = []string{"facebook"}
			f.Media = []string{"1.jpg"}
			f.ThreadsText = "hola"
		}, field: "threads_text"},
		"story with no file": {id: "p", mutate: func(f *Fields) {
			f.Type = "story"
			f.Media = nil
		}, field: "media"},
		"story with a png": {id: "p", mutate: func(f *Fields) {
			f.Type = "story"
			f.Media = []string{"1.png"}
		}, field: "media"},
		"story with a cover": {id: "p", mutate: func(f *Fields) {
			f.Type = "story"
			f.Media = []string{"1.jpg"}
			f.Cover = "portada.jpg"
		}, field: "cover"},
		"story caption too long": {id: "p", mutate: func(f *Fields) {
			f.Type = "story"
			f.Media = []string{"1.jpg"}
			f.Caption = strings.Repeat("x", 2201)
		}, field: "caption"},
		"empty caption":     {id: "p", mutate: func(f *Fields) { f.Caption = "   " }, field: "caption"},
		"caption too long":  {id: "p", mutate: func(f *Fields) { f.Caption = strings.Repeat("x", 2201) }, field: "caption"},
		"alt text too long": {id: "p", mutate: func(f *Fields) { f.AltText = strings.Repeat("x", 1001) }, field: "alt_text"},
		"image with two": {id: "p", mutate: func(f *Fields) {
			f.Media = []string{"1.jpg", "2.jpg"}
		}, field: "media"},
		"carousel with one": {id: "p", mutate: func(f *Fields) {
			f.Type = "carousel"
		}, field: "media"},
		"carousel with eleven": {id: "p", mutate: func(f *Fields) {
			f.Type = "carousel"
			f.Media = make([]string, 11)
			for i := range f.Media {
				f.Media[i] = "m" + strings.Repeat("x", i) + ".jpg"
			}
		}, field: "media"},
		"reel with jpg": {id: "p", mutate: func(f *Fields) {
			f.Type = "reel"
			f.Media = []string{"1.jpg"}
		}, field: "media"},
		"image with mp4": {id: "p", mutate: func(f *Fields) { f.Media = []string{"1.mp4"} }, field: "media"},
		"media with slash": {id: "p", mutate: func(f *Fields) {
			f.Media = []string{"sub/1.jpg"}
		}, field: "media"},
		"duplicate media": {id: "p", mutate: func(f *Fields) {
			f.Type = "carousel"
			f.Media = []string{"1.jpg", "1.jpg"}
		}, field: "media"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := validFields()
			if tc.mutate != nil {
				tc.mutate(&f)
			}
			_, errs := ValidatePost(tc.id, f)
			if errs == nil {
				t.Fatalf("ValidatePost() accepted a malformed post")
			}
			if _, ok := errs[tc.field]; !ok {
				t.Fatalf("errors = %v, want key %q", errs, tc.field)
			}
		})
	}
}

func TestCheckMediaUpload(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name, contentType string
		size              int64
		want              error
		format            MediaFormat
	}{
		"jpeg image":    {name: "1.jpg", contentType: "image/jpeg", size: 10, format: FormatJPEG},
		"jpeg extended": {name: "1.jpeg", contentType: "image/jpeg; charset=binary", size: 10, format: FormatJPEG},
		"mp4 video":     {name: "clip.mp4", contentType: "video/mp4", size: 10, format: FormatMP4},
		"png refused":   {name: "1.png", contentType: "image/png", size: 10, want: ErrMediaType},
		"type mismatch": {name: "1.jpg", contentType: "video/mp4", size: 10, want: ErrMediaType},
		"bad name":      {name: "../1.jpg", contentType: "image/jpeg", size: 10, want: ErrMediaName},
		"empty body":    {name: "1.jpg", contentType: "image/jpeg", size: 0, want: ErrMediaSize},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			format, err := CheckMediaUpload(tc.name, tc.contentType, tc.size)
			if !errors.Is(err, tc.want) {
				t.Fatalf("CheckMediaUpload() err = %v, want %v", err, tc.want)
			}
			if tc.want == nil && format != tc.format {
				t.Fatalf("format = %q, want %q", format, tc.format)
			}
		})
	}
}

func TestMediaSizeLimits(t *testing.T) {
	t.Parallel()

	if _, err := CheckMediaUpload("1.jpg", "image/jpeg", MaxImageBytes+1); !errors.Is(err, ErrMediaTooLarge) {
		t.Fatalf("oversized image: err = %v, want ErrMediaTooLarge", err)
	}
	if _, err := CheckMediaUpload("clip.mp4", "video/mp4", MaxVideoBytes); err != nil {
		t.Fatalf("video at the cap: err = %v, want nil", err)
	}
}

func TestSniffMatchesRejectsWrongBytes(t *testing.T) {
	t.Parallel()

	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0x10, 'J', 'F', 'I', 'F'}
	mp4 := []byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}
	if !SniffMatches(FormatJPEG, jpeg) || SniffMatches(FormatJPEG, mp4) {
		t.Fatal("JPEG sniff must accept JPEG bytes and refuse an MP4 header")
	}
	if !SniffMatches(FormatMP4, mp4) || SniffMatches(FormatMP4, jpeg) {
		t.Fatal("MP4 sniff must accept an ftyp box and refuse JPEG bytes")
	}
}
