// Package social publishes Vibe's own marketing posts to its Facebook Page and
// its Instagram professional account. Posts arrive from a private content
// repository through an OIDC-authenticated sync, are frozen once any network
// has published them, and are published by a recurring job.
package social

import (
	"bytes"
	"errors"
	"fmt"
	"mime"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stodulski/vibe-server/internal/validator"
)

// Network is a platform a post is published to.
type Network string

// The two networks a post can name.
const (
	NetworkInstagram Network = "instagram"
	NetworkFacebook  Network = "facebook"
)

// Kind is the shape of a post: one image, a carousel of images, or one reel.
type Kind string

// The three post shapes.
const (
	TypeImage    Kind = "image"
	TypeCarousel Kind = "carousel"
	TypeReel     Kind = "reel"
)

// Status is where a post is in its life. Only scheduled, publishing and partial
// posts are live; the rest are final or waiting for an owner.
type Status string

// The post statuses, matching the social_posts status check.
const (
	StatusScheduled  Status = "scheduled"
	StatusPublishing Status = "publishing"
	StatusPublished  Status = "published"
	StatusPartial    Status = "partial"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

// Limits the sync is held to. Meta enforces its own on top of these.
const (
	MaxCaptionRunes        = 2200
	MaxAltTextRunes        = 1000
	MinCarouselItems       = 2
	MaxCarouselItems       = 10
	MaxImageBytes    int64 = 8 << 20
	MaxVideoBytes    int64 = 100 << 20
)

// MediaFormat is the container of one uploaded file.
type MediaFormat string

// The two media formats the sync may upload.
const (
	FormatJPEG MediaFormat = "jpeg"
	FormatMP4  MediaFormat = "mp4"
)

// Sentinel errors. Handlers and the publisher branch on them with errors.Is.
var (
	// ErrFrozen means a post's content may no longer change: a network has
	// published it, or a publish run holds it.
	ErrFrozen = errors.New("social: post is frozen")
	// ErrMediaName means an upload's file name is not a plain, allowed name.
	ErrMediaName = errors.New("social: media name")
	// ErrMediaType means an upload's content type or format is not allowed.
	ErrMediaType = errors.New("social: media type")
	// ErrMediaSize means an upload is empty.
	ErrMediaSize = errors.New("social: media size")
	// ErrMediaTooLarge means an upload exceeds its format's cap.
	ErrMediaTooLarge = errors.New("social: media too large")
	// ErrPermanent wraps a platform refusal that retrying cannot fix.
	ErrPermanent = errors.New("social: permanent platform refusal")
)

var (
	postIDPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,80}$`)
	mediaNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
)

// Fields is the content of one post as the sync sends it, before validation.
type Fields struct {
	PublishAt string
	Networks  []string
	Type      string
	Caption   string
	AltText   string
	Media     []string
	Draft     bool
}

// Media is one uploaded file of a post, stored under a content-hash key.
type Media struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	SHA256      string `json:"sha256"`
	SizeBytes   int64  `json:"size_bytes"`
}

// Post is a validated marketing post and its publishing state.
type Post struct {
	ID            string
	PublishAt     time.Time
	Networks      []Network
	Kind          Kind
	Caption       string
	AltText       string
	Media         []Media
	Draft         bool
	Status        Status
	IGMediaID     string
	FBPostID      string
	Attempts      int
	LastError     string
	NextAttemptAt *time.Time
	PublishedAt   *time.Time
}

// ValidatePost checks the content of one post and returns it ready to store.
// Media holds names only; the caller fills in the uploaded records. The map is
// nil when the post is valid, and otherwise maps each field to its message.
func ValidatePost(id string, f Fields) (Post, map[string]string) {
	v := validator.New()
	v.Check(postIDPattern.MatchString(id), "id", "must be lowercase letters, digits and hyphens, 81 characters at most")

	publishAt, err := time.Parse(time.RFC3339, strings.TrimSpace(f.PublishAt))
	v.Check(err == nil, "publish_at", "must be an RFC 3339 time with an offset")

	networks := validateNetworks(v, f.Networks)

	kind := Kind(strings.TrimSpace(f.Type))
	v.Check(validKind(kind), "type", "must be image, carousel or reel")

	caption := strings.TrimSpace(f.Caption)
	v.Check(caption != "", "caption", "must be provided")
	v.Check(utf8.RuneCountInString(caption) <= MaxCaptionRunes, "caption", "must be 2200 characters at most")

	alt := strings.TrimSpace(f.AltText)
	v.Check(utf8.RuneCountInString(alt) <= MaxAltTextRunes, "alt_text", "must be 1000 characters at most")

	media := validateMediaNames(v, kind, f.Media)

	if !v.Valid() {
		return Post{}, v.Errors
	}
	return Post{
		ID:            id,
		PublishAt:     publishAt,
		Networks:      networks,
		Kind:          kind,
		Caption:       caption,
		AltText:       alt,
		Media:         media,
		Draft:         f.Draft,
		Status:        StatusScheduled,
		NextAttemptAt: &publishAt,
	}, nil
}

func validateNetworks(v *validator.Validator, raw []string) []Network {
	v.Check(len(raw) > 0, "networks", "must name at least one network")
	networks := make([]Network, 0, len(raw))
	for _, r := range raw {
		n := Network(strings.TrimSpace(r))
		v.Check(validator.PermittedValue(n, NetworkInstagram, NetworkFacebook), "networks", "must be instagram or facebook")
		networks = append(networks, n)
	}
	v.Check(validator.Unique(networks), "networks", "must not repeat a network")
	return networks
}

func validateMediaNames(v *validator.Validator, kind Kind, raw []string) []Media {
	media := make([]Media, 0, len(raw))
	for _, name := range raw {
		v.Check(mediaNamePattern.MatchString(name), "media", "names must be plain file names")
		v.Check(extensionAllowed(kind, name), "media", "extension does not match the post type")
		media = append(media, Media{Name: name})
	}
	v.Check(validator.Unique(raw), "media", "must not repeat a file")
	v.Check(mediaCountAllowed(kind, len(raw)), "media", mediaCountMessage(kind))
	return media
}

func validKind(kind Kind) bool {
	return validator.PermittedValue(kind, TypeImage, TypeCarousel, TypeReel)
}

func mediaCountAllowed(kind Kind, n int) bool {
	switch kind {
	case TypeImage, TypeReel:
		return n == 1
	case TypeCarousel:
		return n >= MinCarouselItems && n <= MaxCarouselItems
	default:
		return false
	}
}

func mediaCountMessage(kind Kind) string {
	switch kind {
	case TypeImage, TypeReel:
		return "must list exactly one file"
	case TypeCarousel:
		return "must list 2 to 10 files"
	default:
		return "the post type is unknown"
	}
}

func extensionAllowed(kind Kind, name string) bool {
	ext := strings.ToLower(path.Ext(name))
	switch kind {
	case TypeImage, TypeCarousel:
		return ext == ".jpg" || ext == ".jpeg"
	case TypeReel:
		return ext == ".mp4"
	default:
		return false
	}
}

// CheckMediaUpload validates one upload before its body is read. It returns the
// format the file's extension names, which the caller uses to bound the read.
func CheckMediaUpload(name, contentType string, size int64) (MediaFormat, error) {
	if !mediaNamePattern.MatchString(name) {
		return "", fmt.Errorf("%w: %q is not a plain file name", ErrMediaName, name)
	}
	format, ok := formatForName(name)
	if !ok {
		return "", fmt.Errorf("%w: %q is neither JPEG nor MP4", ErrMediaType, name)
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != wantContentType(format) {
		return "", fmt.Errorf("%w: %s must be sent as %s", ErrMediaType, name, wantContentType(format))
	}
	if size <= 0 {
		return "", fmt.Errorf("%w: %s is empty", ErrMediaSize, name)
	}
	if size > SizeLimit(format) {
		return "", fmt.Errorf("%w: %s exceeds %d bytes", ErrMediaTooLarge, name, SizeLimit(format))
	}
	return format, nil
}

// SniffMatches reports whether the first bytes of a body are the container its
// declared format names. It catches a file renamed to the wrong extension.
func SniffMatches(format MediaFormat, body []byte) bool {
	switch format {
	case FormatJPEG:
		return len(body) >= 3 && body[0] == 0xFF && body[1] == 0xD8 && body[2] == 0xFF
	case FormatMP4:
		return len(body) >= 8 && bytes.Equal(body[4:8], []byte("ftyp"))
	default:
		return false
	}
}

// MediaKey is the object key a media file is stored under. The key is the
// content hash, so the same bytes always land on the same key.
func MediaKey(sha256Hex string, format MediaFormat) string {
	ext := "jpg"
	if format == FormatMP4 {
		ext = "mp4"
	}
	return "social/" + sha256Hex + "." + ext
}

func formatForName(name string) (MediaFormat, bool) {
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg":
		return FormatJPEG, true
	case ".mp4":
		return FormatMP4, true
	default:
		return "", false
	}
}

func wantContentType(format MediaFormat) string {
	switch format {
	case FormatJPEG:
		return "image/jpeg"
	case FormatMP4:
		return "video/mp4"
	default:
		return ""
	}
}

// SizeLimit is the largest body a format may upload.
func SizeLimit(format MediaFormat) int64 {
	switch format {
	case FormatJPEG:
		return MaxImageBytes
	case FormatMP4:
		return MaxVideoBytes
	default:
		return 0
	}
}
