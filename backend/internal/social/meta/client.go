// Package meta is the Graph API client the social publisher uses to publish
// Vibe's posts to its Facebook Page and its Instagram professional account.
package meta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
	"github.com/stodulski/vibe-server/internal/social"
)

const (
	defaultAPIBase    = "https://graph.facebook.com/v25.0"
	defaultUploadBase = "https://rupload.facebook.com/video-upload/v25.0"
	defaultTimeout    = 30 * time.Second
	defaultPollEvery  = 30 * time.Second
	defaultPollFor    = 5 * time.Minute
	maxResponseBytes  = 1 << 20
	// instagramDailyLimit is Meta's documented number of containers an
	// account may publish in a rolling 24 hours. It is the fallback when the
	// quota answer omits its total.
	instagramDailyLimit = 50
)

// Config is the Meta side of the publisher. Token is a system-user token and
// is used for Instagram and for finding the Page. Facebook Page posts use the
// Page token that the Page lookup returns. Zero fields take the defaults shown
// by the constants above.
type Config struct {
	Token      string
	PageID     string
	APIBase    string
	UploadBase string
	HTTP       *http.Client
	CB         *circuitbreaker.CircuitBreaker
	PollEvery  time.Duration
	PollFor    time.Duration
}

// Client publishes posts through the Graph API.
type Client struct {
	cfg  Config
	http *http.Client
	cb   *circuitbreaker.CircuitBreaker

	mu   sync.Mutex
	acct *account
}

type account struct {
	pageID    string
	pageToken string
	igUserID  string
}

// New returns a Client for cfg.
func New(cfg Config) *Client {
	if cfg.APIBase == "" {
		cfg.APIBase = defaultAPIBase
	}
	if cfg.UploadBase == "" {
		cfg.UploadBase = defaultUploadBase
	}
	if cfg.PollEvery <= 0 {
		cfg.PollEvery = defaultPollEvery
	}
	if cfg.PollFor <= 0 {
		cfg.PollFor = defaultPollFor
	}
	httpClient := cfg.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{cfg: cfg, http: httpClient, cb: cfg.CB}
}

// InstagramQuota returns the containers published in the last 24 hours and
// the most the account may publish in that window.
func (c *Client) InstagramQuota(ctx context.Context) (int, int, error) {
	acct, err := c.account(ctx)
	if err != nil {
		return 0, 0, err
	}
	var out struct {
		Data []struct {
			QuotaUsage int `json:"quota_usage"`
			Config     struct {
				QuotaTotal int `json:"quota_total"`
			} `json:"config"`
		} `json:"data"`
	}
	if err := c.get(ctx, c.cfg.Token, "/"+acct.igUserID+"/content_publishing_limit", url.Values{"fields": {"quota_usage,config"}}, &out); err != nil {
		return 0, 0, err
	}
	if len(out.Data) == 0 {
		return 0, 0, errors.New("meta: content_publishing_limit answered no quota")
	}
	total := out.Data[0].Config.QuotaTotal
	if total <= 0 {
		total = instagramDailyLimit
	}
	return out.Data[0].QuotaUsage, total, nil
}

// PublishInstagram publishes post to the Instagram account and returns the
// media id. The container must report FINISHED before it is published.
func (c *Client) PublishInstagram(ctx context.Context, post social.Post) (string, error) {
	acct, err := c.account(ctx)
	if err != nil {
		return "", err
	}
	creation, err := c.instagramContainer(ctx, acct.igUserID, post)
	if err != nil {
		return "", err
	}
	if err := c.awaitContainer(ctx, creation); err != nil {
		return "", err
	}
	return c.publishContainer(ctx, acct.igUserID, creation)
}

// publishContainer publishes a FINISHED container. Meta can still answer that
// the media is not ready for a short while after FINISHED, so that answer is
// retried within PollFor before it is returned as a retryable error.
func (c *Client) publishContainer(ctx context.Context, igID, creation string) (string, error) {
	deadline := time.Now().Add(c.cfg.PollFor)
	for {
		var out struct {
			ID string `json:"id"`
		}
		err := c.post(ctx, c.cfg.Token, "/"+igID+"/media_publish", url.Values{"creation_id": {creation}}, &out)
		if err == nil {
			return out.ID, nil
		}
		if !errors.Is(err, errMediaNotReady) || time.Now().After(deadline) {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("wait for instagram media: %w", ctx.Err())
		case <-time.After(c.cfg.PollEvery):
		}
	}
}

// InstagramPermalink returns the public URL of a published Instagram media.
func (c *Client) InstagramPermalink(ctx context.Context, mediaID string) (string, error) {
	var out struct {
		Permalink string `json:"permalink"`
	}
	if err := c.get(ctx, c.cfg.Token, "/"+mediaID, url.Values{"fields": {"permalink"}}, &out); err != nil {
		return "", err
	}
	return out.Permalink, nil
}

// PublishFacebook publishes post to the Page and returns the post id. A reel
// is the Page's video_reels session, and its id is the video id.
func (c *Client) PublishFacebook(ctx context.Context, post social.Post) (string, error) {
	acct, err := c.account(ctx)
	if err != nil {
		return "", err
	}
	switch post.Kind {
	case social.TypeImage:
		return c.facebookPhoto(ctx, acct, post)
	case social.TypeCarousel:
		return c.facebookCarousel(ctx, acct, post)
	case social.TypeReel:
		return c.facebookReel(ctx, acct, post)
	case social.TypeStory:
		return c.facebookStory(ctx, acct, post)
	case social.TypeText:
		return "", fmt.Errorf("%w: text posts go to threads only", social.ErrPermanent)
	default:
		return "", fmt.Errorf("%w: unknown post kind %q", social.ErrPermanent, post.Kind)
	}
}

func (c *Client) instagramContainer(ctx context.Context, igID string, post social.Post) (string, error) {
	switch post.Kind {
	case social.TypeStory:
		return c.createContainer(ctx, igID, instagramStoryForm(post.Media[0]))
	case social.TypeImage:
		form := url.Values{"image_url": {post.Media[0].URL}, "caption": {post.Caption}}
		if post.AltText != "" {
			form.Set("alt_text", post.AltText)
		}
		return c.createContainer(ctx, igID, form)
	case social.TypeCarousel:
		return c.instagramCarousel(ctx, igID, post)
	case social.TypeReel:
		form := url.Values{"media_type": {"REELS"}, "video_url": {post.Media[0].URL}, "caption": {post.Caption}, "share_to_feed": {"true"}}
		if post.Cover != nil {
			form.Set("cover_url", post.Cover.URL)
		}
		return c.createContainer(ctx, igID, form)
	case social.TypeText:
		return "", fmt.Errorf("%w: text posts go to threads only", social.ErrPermanent)
	default:
		return "", fmt.Errorf("%w: unknown post kind %q", social.ErrPermanent, post.Kind)
	}
}

func (c *Client) instagramCarousel(ctx context.Context, igID string, post social.Post) (string, error) {
	children := make([]string, 0, len(post.Media))
	for _, m := range post.Media {
		id, err := c.createContainer(ctx, igID, url.Values{"image_url": {m.URL}, "is_carousel_item": {"true"}})
		if err != nil {
			return "", err
		}
		children = append(children, id)
	}
	// Every child must be FINISHED before the parent is built, or the parent
	// can report FINISHED while media_publish still refuses it as not ready.
	for _, id := range children {
		if err := c.awaitContainer(ctx, id); err != nil {
			return "", err
		}
	}
	form := url.Values{"media_type": {"CAROUSEL"}, "children": {strings.Join(children, ",")}, "caption": {post.Caption}}
	return c.createContainer(ctx, igID, form)
}

func (c *Client) createContainer(ctx context.Context, igID string, form url.Values) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	if err := c.post(ctx, c.cfg.Token, "/"+igID+"/media", form, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// awaitContainer polls a container until it is FINISHED. Meta recommends a
// check about once a minute for no more than five, so the wait is bounded by
// PollFor. ERROR and EXPIRED are retryable: a fresh attempt makes a new container.
func (c *Client) awaitContainer(ctx context.Context, id string) error {
	deadline := time.Now().Add(c.cfg.PollFor)
	for {
		var st struct {
			StatusCode string `json:"status_code"`
		}
		if err := c.get(ctx, c.cfg.Token, "/"+id, url.Values{"fields": {"status_code"}}, &st); err != nil {
			return err
		}
		switch st.StatusCode {
		case "FINISHED":
			return nil
		case "ERROR", "EXPIRED":
			return fmt.Errorf("instagram container %s ended as %s", id, st.StatusCode)
		case "IN_PROGRESS", "":
			// still processing: keep waiting below
		default:
			return fmt.Errorf("instagram container %s has unexpected status %q", id, st.StatusCode)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("instagram container %s still processing after %s", id, c.cfg.PollFor)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for instagram container: %w", ctx.Err())
		case <-time.After(c.cfg.PollEvery):
		}
	}
}

func (c *Client) facebookPhoto(ctx context.Context, acct account, post social.Post) (string, error) {
	form := url.Values{"url": {post.Media[0].URL}, "message": {post.Caption}}
	if post.AltText != "" {
		form.Set("alt_text_custom", post.AltText)
	}
	var out struct {
		ID     string `json:"id"`
		PostID string `json:"post_id"`
	}
	if err := c.post(ctx, acct.pageToken, "/"+acct.pageID+"/photos", form, &out); err != nil {
		return "", err
	}
	if out.PostID != "" {
		return out.PostID, nil
	}
	return out.ID, nil
}

func (c *Client) facebookCarousel(ctx context.Context, acct account, post social.Post) (string, error) {
	form := url.Values{"message": {post.Caption}}
	for i, m := range post.Media {
		var up struct {
			ID string `json:"id"`
		}
		if err := c.post(ctx, acct.pageToken, "/"+acct.pageID+"/photos", url.Values{"url": {m.URL}, "published": {"false"}}, &up); err != nil {
			return "", err
		}
		attached := fmt.Sprintf(`{"media_fbid":%q}`, up.ID)
		form.Set(fmt.Sprintf("attached_media[%d]", i), attached)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := c.post(ctx, acct.pageToken, "/"+acct.pageID+"/feed", form, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// facebookReel runs the Page reel session: start, upload from the hosted URL,
// then finish with PUBLISHED. Each step names itself in its error.
func (c *Client) facebookReel(ctx context.Context, acct account, post social.Post) (string, error) {
	var start struct {
		VideoID string `json:"video_id"`
	}
	if err := c.post(ctx, acct.pageToken, "/"+acct.pageID+"/video_reels", url.Values{"upload_phase": {"start"}}, &start); err != nil {
		return "", fmt.Errorf("reel start: %w", err)
	}
	if start.VideoID == "" {
		return "", errors.New("reel start: meta returned no video id")
	}
	accepted, err := c.uploadHostedVideo(ctx, acct.pageToken, start.VideoID, post.Media[0].URL)
	if err != nil {
		return "", fmt.Errorf("reel upload: %w", err)
	}
	if !accepted {
		return "", errors.New("reel upload: meta did not accept the upload")
	}

	form := url.Values{
		"upload_phase": {"finish"}, "video_id": {start.VideoID},
		"video_state": {"PUBLISHED"}, "description": {post.Caption},
	}
	var fin struct {
		Success bool `json:"success"`
	}
	if err := c.post(ctx, acct.pageToken, "/"+acct.pageID+"/video_reels", form, &fin); err != nil {
		return "", fmt.Errorf("reel finish: %w", err)
	}
	if !fin.Success {
		return "", errors.New("reel finish: meta did not accept the publish")
	}
	return start.VideoID, nil
}

// uploadHostedVideo asks Meta to fetch fileURL into an open video session. The
// answer is the upload's success flag; a transport or status failure is an error.
func (c *Client) uploadHostedVideo(ctx context.Context, token, videoID, fileURL string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.UploadBase+"/"+videoID, nil)
	if err != nil {
		return false, fmt.Errorf("meta: build video upload: %w", err)
	}
	req.Header.Set("Authorization", "OAuth "+token)
	req.Header.Set("file_url", fileURL)
	var up struct {
		Success bool `json:"success"`
	}
	if err := c.send(req, &up); err != nil {
		return false, err
	}
	return up.Success, nil
}

// facebookStory publishes a Page story. A photo is one unpublished upload and a
// photo story; a video is the same three-step session a reel uses.
func (c *Client) facebookStory(ctx context.Context, acct account, post social.Post) (string, error) {
	if post.Media[0].IsVideo() {
		return c.facebookVideoStory(ctx, acct, post)
	}
	return c.facebookPhotoStory(ctx, acct, post)
}

func (c *Client) facebookPhotoStory(ctx context.Context, acct account, post social.Post) (string, error) {
	var up struct {
		ID string `json:"id"`
	}
	if err := c.post(ctx, acct.pageToken, "/"+acct.pageID+"/photos", url.Values{"url": {post.Media[0].URL}, "published": {"false"}}, &up); err != nil {
		return "", err
	}
	var out struct {
		Success bool    `json:"success"`
		PostID  graphID `json:"post_id"`
	}
	if err := c.post(ctx, acct.pageToken, "/"+acct.pageID+"/photo_stories", url.Values{"photo_id": {up.ID}}, &out); err != nil {
		return "", err
	}
	if !out.Success || out.PostID == "" {
		return "", errors.New("meta: photo story was not accepted")
	}
	return string(out.PostID), nil
}

func (c *Client) facebookVideoStory(ctx context.Context, acct account, post social.Post) (string, error) {
	var start struct {
		VideoID string `json:"video_id"`
	}
	if err := c.post(ctx, acct.pageToken, "/"+acct.pageID+"/video_stories", url.Values{"upload_phase": {"start"}}, &start); err != nil {
		return "", fmt.Errorf("video story start: %w", err)
	}
	if start.VideoID == "" {
		return "", errors.New("video story start: meta returned no video id")
	}
	accepted, err := c.uploadHostedVideo(ctx, acct.pageToken, start.VideoID, post.Media[0].URL)
	if err != nil {
		return "", fmt.Errorf("video story upload: %w", err)
	}
	if !accepted {
		return "", errors.New("video story upload: meta did not accept the upload")
	}
	var fin struct {
		Success bool    `json:"success"`
		PostID  graphID `json:"post_id"`
	}
	if err := c.post(ctx, acct.pageToken, "/"+acct.pageID+"/video_stories", url.Values{"upload_phase": {"finish"}, "video_id": {start.VideoID}}, &fin); err != nil {
		return "", fmt.Errorf("video story finish: %w", err)
	}
	if !fin.Success || fin.PostID == "" {
		return "", errors.New("video story finish: meta did not accept the publish")
	}
	return string(fin.PostID), nil
}

// instagramStoryForm is the container for an Instagram story. Stories take
// neither a caption nor alt text, so only the media is sent.
func instagramStoryForm(m social.Media) url.Values {
	if m.IsVideo() {
		return url.Values{"media_type": {"STORIES"}, "video_url": {m.URL}}
	}
	return url.Values{"media_type": {"STORIES"}, "image_url": {m.URL}}
}

// graphID is an id Graph answers either as a JSON string or as a bare number,
// as the Page story endpoints do for post_id.
type graphID string

func (g *graphID) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*g = ""
		return nil
	}
	if len(raw) > 0 && raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return fmt.Errorf("meta: decode post_id: %w", err)
		}
		*g = graphID(s)
		return nil
	}
	*g = graphID(raw)
	return nil
}

// account finds the Page and its Instagram account once per process and keeps
// the answer. The Page token it returns is used for Page posts only.
func (c *Client) account(ctx context.Context) (account, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.acct != nil {
		return *c.acct, nil
	}

	var out struct {
		Data []pageEntry `json:"data"`
	}
	if err := c.get(ctx, c.cfg.Token, "/me/accounts", url.Values{"fields": {"id,name,access_token,instagram_business_account"}}, &out); err != nil {
		return account{}, err
	}
	pages := out.Data
	if c.cfg.PageID != "" {
		pages = filterPages(pages, c.cfg.PageID)
	}
	if len(pages) != 1 {
		return account{}, fmt.Errorf("%w: the token reaches %d pages; set META_SOCIAL_PAGE_ID to choose one", social.ErrPermanent, len(pages))
	}
	page := pages[0]
	if page.InstagramBusinessAccount == nil || page.InstagramBusinessAccount.ID == "" {
		return account{}, fmt.Errorf("%w: page %s has no Instagram professional account connected", social.ErrPermanent, page.ID)
	}
	acct := account{pageID: page.ID, pageToken: page.AccessToken, igUserID: page.InstagramBusinessAccount.ID}
	c.acct = &acct
	return acct, nil
}

// pageEntry is one Page the token can act on, as /me/accounts lists it.
type pageEntry struct {
	ID                       string `json:"id"`
	AccessToken              string `json:"access_token"`
	InstagramBusinessAccount *struct {
		ID string `json:"id"`
	} `json:"instagram_business_account"`
}

func filterPages(pages []pageEntry, id string) []pageEntry {
	var out []pageEntry
	for _, p := range pages {
		if p.ID == id {
			out = append(out, p)
		}
	}
	return out
}

// get and post are the two request shapes. The token is always a parameter,
// never a header, so one helper covers the Graph API and its upload host alike.
func (c *Client) get(ctx context.Context, token, path string, params url.Values, out any) error {
	q := withToken(params, token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.APIBase+path+"?"+q.Encode(), nil)
	if err != nil {
		return fmt.Errorf("meta: build request: %w", err)
	}
	return c.send(req, out)
}

func (c *Client) post(ctx context.Context, token, path string, form url.Values, out any) error {
	body := withToken(form, token).Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.APIBase+path, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("meta: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.send(req, out)
}

// send is the one place a request leaves this package. It is guarded by the
// breaker, the response is read to the end, and any failure is returned with
// the URL and the token removed.
func (c *Client) send(req *http.Request, out any) error {
	if err := c.cb.AllowRequest(); err != nil {
		return fmt.Errorf("meta: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.cb.RecordFailure()
		return redactTransport(err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		c.cb.RecordFailure()
		return fmt.Errorf("meta: read response: %w", err)
	}
	if transient(resp.StatusCode) {
		c.cb.RecordFailure()
	} else {
		c.cb.RecordSuccess()
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return apiError(resp.StatusCode, raw)
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("meta: decode response: %w", err)
		}
	}
	return nil
}

func withToken(params url.Values, token string) url.Values {
	q := maps.Clone(params)
	if q == nil {
		q = url.Values{}
	}
	q.Set("access_token", token)
	return q
}

func transient(status int) bool {
	return status >= http.StatusInternalServerError || status == http.StatusTooManyRequests
}

// graphErrorBody is Graph's error answer. Error is nil when the body has no
// error object. A rupload answer has none and carries debug_info instead.
type graphErrorBody struct {
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
	DebugInfo *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"debug_info"`
}

// codeMediaNotReady is Graph's answer to media_publish while the media is still
// processing (code 9007, subcode 2207027). Meta documents it as wait and retry.
const codeMediaNotReady = 9007

// errMediaNotReady marks that answer. It is retryable, not permanent.
var errMediaNotReady = errors.New("instagram media is not ready")

// apiError turns an error answer into an error. A Graph error object keeps its
// message as before. Any other body adds a redacted snippet of the body, so the
// refusal can be read. A client error other than 429 or media not ready is
// permanent, because the same request would fail the same way again.
func apiError(status int, raw []byte) error {
	var body graphErrorBody
	if err := json.Unmarshal(raw, &body); err != nil {
		body = graphErrorBody{}
	}
	code, msg := 0, ""
	switch {
	case body.Error != nil:
		code, msg = body.Error.Code, body.Error.Message
	case body.DebugInfo != nil:
		msg = redactToken(debugText(body.DebugInfo.Type, body.DebugInfo.Message))
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	text := fmt.Sprintf("meta answered %d (code %d): %s", status, code, msg)
	if body.Error == nil {
		if snippet := bodySnippet(raw); snippet != "" {
			text += ": body " + snippet
		}
	}
	if code == codeMediaNotReady {
		return fmt.Errorf("%w: %s", errMediaNotReady, text)
	}
	if status >= http.StatusBadRequest && status < http.StatusInternalServerError && status != http.StatusTooManyRequests {
		return fmt.Errorf("%w: %s", social.ErrPermanent, text)
	}
	return errors.New(text)
}

// debugText joins a rupload debug_info type and message as "type: message".
// Whichever of the two is present is used alone.
func debugText(kind, message string) string {
	switch {
	case kind != "" && message != "":
		return kind + ": " + message
	case message != "":
		return message
	default:
		return kind
	}
}

// maxSnippetBytes bounds the body text an error carries, ellipsis included.
const maxSnippetBytes = 300

// bodySnippet returns the body as one line with any access token redacted. It
// is cut to at most maxSnippetBytes on a rune boundary.
func bodySnippet(raw []byte) string {
	s := redactToken(strings.Join(strings.Fields(string(raw)), " "))
	if len(s) <= maxSnippetBytes {
		return s
	}
	const ellipsis = "…"
	cut := maxSnippetBytes - len(ellipsis)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + ellipsis
}

// tokenParam and tokenMember find an access token echoed in a body, as a query
// pair or as a JSON member. The value is replaced, never kept.
var (
	tokenParam  = regexp.MustCompile(`access_token=[^&\s"]*`)
	tokenMember = regexp.MustCompile(`("access_token"\s*:\s*")[^"]*`)
)

func redactToken(s string) string {
	s = tokenParam.ReplaceAllString(s, "access_token=[redacted]")
	return tokenMember.ReplaceAllString(s, `${1}[redacted]`)
}

// redactTransport keeps the failure and drops the URL, which carries the
// access token as a query parameter.
func redactTransport(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("meta request %s failed: %w", urlErr.Op, urlErr.Err)
	}
	return fmt.Errorf("meta request failed: %w", err)
}
