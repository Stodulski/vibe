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
	"strings"
	"sync"
	"time"

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
	var out struct {
		ID string `json:"id"`
	}
	if err := c.post(ctx, c.cfg.Token, "/"+acct.igUserID+"/media_publish", url.Values{"creation_id": {creation}}, &out); err != nil {
		return "", err
	}
	return out.ID, nil
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
	default:
		return "", fmt.Errorf("%w: unknown post kind %q", social.ErrPermanent, post.Kind)
	}
}

func (c *Client) instagramContainer(ctx context.Context, igID string, post social.Post) (string, error) {
	switch post.Kind {
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
		return c.createContainer(ctx, igID, form)
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
// then finish with PUBLISHED.
func (c *Client) facebookReel(ctx context.Context, acct account, post social.Post) (string, error) {
	var start struct {
		VideoID string `json:"video_id"`
	}
	if err := c.post(ctx, acct.pageToken, "/"+acct.pageID+"/video_reels", url.Values{"upload_phase": {"start"}}, &start); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.UploadBase+"/"+start.VideoID, nil)
	if err != nil {
		return "", fmt.Errorf("meta: build reel upload: %w", err)
	}
	req.Header.Set("Authorization", "OAuth "+acct.pageToken)
	req.Header.Set("file_url", post.Media[0].URL)
	var up struct {
		Success bool `json:"success"`
	}
	if err := c.send(req, &up); err != nil {
		return "", err
	}
	if !up.Success {
		return "", errors.New("meta: reel upload was not accepted")
	}

	form := url.Values{
		"upload_phase": {"finish"}, "video_id": {start.VideoID},
		"video_state": {"PUBLISHED"}, "description": {post.Caption},
	}
	var fin struct {
		Success bool `json:"success"`
	}
	if err := c.post(ctx, acct.pageToken, "/"+acct.pageID+"/video_reels", form, &fin); err != nil {
		return "", err
	}
	if !fin.Success {
		return "", errors.New("meta: reel publish was not accepted")
	}
	return start.VideoID, nil
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

type graphErrorBody struct {
	Error struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// apiError turns a Graph error answer into an error. A client error other than
// 429 is permanent, because the same request would fail the same way again.
func apiError(status int, raw []byte) error {
	var body graphErrorBody
	_ = json.Unmarshal(raw, &body)
	msg := body.Error.Message
	if msg == "" {
		msg = http.StatusText(status)
	}
	text := fmt.Sprintf("meta answered %d (code %d): %s", status, body.Error.Code, msg)
	if status >= http.StatusBadRequest && status < http.StatusInternalServerError && status != http.StatusTooManyRequests {
		return fmt.Errorf("%w: %s", social.ErrPermanent, text)
	}
	return errors.New(text)
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
