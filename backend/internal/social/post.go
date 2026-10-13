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
	"slices"
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
	NetworkThreads   Network = "threads"
)

// Kind is the shape of a post: one image, a carousel of images, one reel, one
// story, or a text-only Threads post.
type Kind string

// The five post shapes. A text post goes to Threads alone; a story goes to
// Instagram or Facebook, never Threads.
const (
	TypeImage    Kind = "image"
	TypeCarousel Kind = "carousel"
	TypeReel     Kind = "reel"
	TypeStory    Kind = "story"
	TypeText     Kind = "text"
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
	// MaxThreadsTextLength is Threads' post limit, measured by ThreadsLength.
	MaxThreadsTextLength = 500
	// MaxThreadsTopicLength is the longest topic tag Threads accepts.
	MaxThreadsTopicLength = 50
)

// StoryGap is the time between one story of a sequence and the next, so the
// stories land in the tray in the order the post lists them.
const StoryGap = 60 * time.Second

// MaxStoryFiles is the most files one story sequence may hold.
const MaxStoryFiles = 6

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
	PublishAt    string
	Networks     []string
	Type         string
	Caption      string
	AltText      string
	Media        []string
	Draft        bool
	Cover        string
	ThreadsText  string
	ThreadsTopic string
	After        string
}

// Media is one uploaded file of a post, stored under a content-hash key.
type Media struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	SHA256      string `json:"sha256"`
	SizeBytes   int64  `json:"size_bytes"`
}

// IsVideo reports whether the file is an MP4. The extension is checked at
// validation, so the name says which container the file is.
func (m Media) IsVideo() bool {
	return strings.EqualFold(path.Ext(m.Name), ".mp4")
}

// StoryItem is one story of a sequence: its place, its time, and the id each
// network gave it once it went out. Item i is the post's Media[i].
type StoryItem struct {
	Position    int       `json:"position"`
	ScheduledAt time.Time `json:"scheduled_at"`
	IGMediaID   string    `json:"ig_media_id,omitempty"`
	FBPostID    string    `json:"fb_post_id,omitempty"`
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
	// Cover is the reel cover, a JPEG Instagram shows on the Reels tab. It is
	// not one of Media, so a reel's media count stays one video.
	Cover *Media
	// ThreadsText is the Threads text. Empty means the caption is the text.
	ThreadsText string
	// ThreadsTopic is the Threads topic tag, empty for none.
	ThreadsTopic   string
	ThreadsMediaID string
	// StoryItems holds one entry per file of a story, in publish order. It is
	// empty for every other type.
	StoryItems []StoryItem
	// After is the id of the feed post a story sequence waits for on each
	// network. Empty means the sequence does not wait.
	After string
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
	v.Check(validKind(kind), "type", "must be image, carousel, reel, text or story")
	if kind == TypeText {
		v.Check(isThreadsOnly(networks), "type", "a text post goes to threads alone")
	}
	if kind == TypeStory {
		v.Check(!slices.Contains(networks, NetworkThreads), "networks", "a story goes to instagram or facebook only")
	}

	caption := strings.TrimSpace(f.Caption)
	if kind != TypeStory {
		v.Check(caption != "", "caption", "must be provided")
	}
	v.Check(utf8.RuneCountInString(caption) <= MaxCaptionRunes, "caption", "must be 2200 characters at most")

	alt := strings.TrimSpace(f.AltText)
	v.Check(utf8.RuneCountInString(alt) <= MaxAltTextRunes, "alt_text", "must be 1000 characters at most")

	media := validateMediaNames(v, kind, f.Media)
	cover := validateCover(v, kind, f.Cover, f.Media)
	threadsText, threadsTopic := validateThreads(v, networks, caption, f.ThreadsText, f.ThreadsTopic)
	after := strings.TrimSpace(f.After)
	validateAfter(v, kind, id, after)

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
		Cover:         cover,
		ThreadsText:   threadsText,
		ThreadsTopic:  threadsTopic,
		StoryItems:    storyItems(kind, publishAt, len(media)),
		After:         after,
	}, nil
}

// validateAfter checks the optional feed-post gate of a story. Only a story may
// name one, and it must name a different, well-formed post id.
func validateAfter(v *validator.Validator, kind Kind, id, after string) {
	if after == "" {
		return
	}
	v.Check(kind == TypeStory, "after", "only a story takes after")
	v.Check(postIDPattern.MatchString(after), "after", "must be a post id")
	v.Check(after != id, "after", "must name another post")
}

// storyItems gives each file of a story its place and its time: the first
// story at the publish time, each next one StoryGap later. Other types have none.
func storyItems(kind Kind, publishAt time.Time, n int) []StoryItem {
	if kind != TypeStory {
		return nil
	}
	items := make([]StoryItem, n)
	for i := range items {
		items[i] = StoryItem{Position: i, ScheduledAt: publishAt.Add(time.Duration(i) * StoryGap)}
	}
	return items
}

func validateNetworks(v *validator.Validator, raw []string) []Network {
	v.Check(len(raw) > 0, "networks", "must name at least one network")
	networks := make([]Network, 0, len(raw))
	for _, r := range raw {
		n := Network(strings.TrimSpace(r))
		v.Check(validator.PermittedValue(n, NetworkInstagram, NetworkFacebook, NetworkThreads), "networks", "must be instagram, facebook or threads")
		networks = append(networks, n)
	}
	v.Check(validator.Unique(networks), "networks", "must not repeat a network")
	return networks
}

func isThreadsOnly(networks []Network) bool {
	return len(networks) == 1 && networks[0] == NetworkThreads
}

// validateCover checks the optional reel cover. It is a plain JPEG name that
// is not also a media file, and it is allowed on reels only.
func validateCover(v *validator.Validator, kind Kind, raw string, media []string) *Media {
	name := strings.TrimSpace(raw)
	if name == "" {
		return nil
	}
	v.Check(kind == TypeReel, "cover", "only a reel takes a cover")
	v.Check(mediaNamePattern.MatchString(name), "cover", "must be a plain file name")
	format, ok := formatForName(name)
	v.Check(ok && format == FormatJPEG, "cover", "must be a JPEG file")
	v.Check(!slices.Contains(media, name), "cover", "must not also be listed as media")
	return &Media{Name: name}
}

// validateThreads checks the Threads fields. The Threads text is threads_text
// or, when that is empty, the caption; either must fit Threads' limit. The
// Threads fields are allowed only when Threads is one of the networks.
func validateThreads(v *validator.Validator, networks []Network, caption, rawText, rawTopic string) (string, string) {
	text := strings.TrimSpace(rawText)
	topic := strings.TrimSpace(rawTopic)
	if !slices.Contains(networks, NetworkThreads) {
		v.Check(text == "", "threads_text", "needs threads in networks")
		v.Check(topic == "", "threads_topic", "needs threads in networks")
		return "", ""
	}
	if text == "" {
		v.Check(ThreadsLength(caption) <= MaxThreadsTextLength, "threads_text", "is required when the caption is longer than 500 characters")
	} else {
		v.Check(ThreadsLength(text) <= MaxThreadsTextLength, "threads_text", "must be 500 characters at most")
	}
	if topic != "" {
		v.Check(ThreadsLength(topic) <= MaxThreadsTopicLength, "threads_topic", "must be 50 characters at most")
		v.Check(!strings.ContainsAny(topic, ".&"), "threads_topic", "must not contain periods or ampersands")
	}
	return text, topic
}

// ThreadsLength measures text the way Threads does: a character outside the
// Basic Multilingual Plane, which covers most emoji, counts as its four UTF-8
// bytes, and every other character counts as one.
func ThreadsLength(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 4
		} else {
			n++
		}
	}
	return n
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
	return validator.PermittedValue(kind, TypeImage, TypeCarousel, TypeReel, TypeStory, TypeText)
}

func mediaCountAllowed(kind Kind, n int) bool {
	switch kind {
	case TypeImage, TypeReel:
		return n == 1
	case TypeStory:
		return n >= 1 && n <= MaxStoryFiles
	case TypeCarousel:
		return n >= MinCarouselItems && n <= MaxCarouselItems
	case TypeText:
		return n == 0
	default:
		return false
	}
}

func mediaCountMessage(kind Kind) string {
	switch kind {
	case TypeImage, TypeReel:
		return "must list exactly one file"
	case TypeStory:
		return "a story lists 1 to 6 files"
	case TypeCarousel:
		return "must list 2 to 10 files"
	case TypeText:
		return "a text post lists no files"
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
	case TypeStory:
		return ext == ".jpg" || ext == ".jpeg" || ext == ".mp4"
	case TypeText:
		return false
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
