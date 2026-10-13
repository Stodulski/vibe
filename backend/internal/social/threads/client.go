// Package threads is the Threads API client the social publisher uses to post
// to Vibe's Threads account, and the OAuth calls that connect it. It is a
// separate client from meta because Threads has its own host, tokens and
// publishing steps.
package threads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
	"github.com/stodulski/vibe-server/internal/social"
)

const (
	defaultBase          = "https://graph.threads.net"
	defaultAuthorizeBase = "https://threads.net"
	apiVersion           = "/v1.0"
	scopes               = "threads_basic,threads_content_publish," + social.ThreadsKeywordScope
	keywordSearchLimit   = 25
	// graphTimeLayout is how Graph writes timestamps: ISO 8601 with a +0000 offset.
	graphTimeLayout  = "2006-01-02T15:04:05-0700"
	defaultPollEvery = 30 * time.Second
	defaultPollFor   = 10 * time.Minute
	defaultTimeout   = 30 * time.Second
	defaultTokenLife = 60 * 24 * time.Hour
	maxResponseBytes = 1 << 20
	// tokenErrorCode is Graph's "invalid or expired access token" code. The post
	// waits for a reconnect instead of ending, so it is never permanent.
	tokenErrorCode = 190
)

// Config is the Threads side of the publisher and the connection flow. Base is
// the Graph host and AuthorizeBase the consent page host; both have defaults.
// Zero durations take the defaults above.
type Config struct {
	AppID         string
	AppSecret     string
	RedirectURI   string
	Base          string
	AuthorizeBase string
	HTTP          *http.Client
	CB            *circuitbreaker.CircuitBreaker
	PollEvery     time.Duration
	PollFor       time.Duration
}

// Client talks to the Threads API for the one connected account.
type Client struct {
	cfg   Config
	http  *http.Client
	cb    *circuitbreaker.CircuitBreaker
	store social.ThreadsStore
}

// New returns a Client that reads the connected account from store.
func New(cfg Config, store social.ThreadsStore) *Client {
	if cfg.Base == "" {
		cfg.Base = defaultBase
	}
	if cfg.AuthorizeBase == "" {
		cfg.AuthorizeBase = defaultAuthorizeBase
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
	return &Client{cfg: cfg, http: httpClient, cb: cfg.CB, store: store}
}

// AuthorizeURL is the consent page the owner grants access on. The state is
// bound to the browser by the caller.
func (c *Client) AuthorizeURL(state string) string {
	q := url.Values{
		"client_id":     {c.cfg.AppID},
		"redirect_uri":  {c.cfg.RedirectURI},
		"scope":         {scopes},
		"response_type": {"code"},
		"state":         {state},
	}
	return c.cfg.AuthorizeBase + "/oauth/authorize?" + q.Encode()
}

// Exchange trades the consent code for a short-lived token and its user id.
func (c *Client) Exchange(ctx context.Context, code string) (social.ThreadsToken, error) {
	var out struct {
		AccessToken string      `json:"access_token"`
		UserID      json.Number `json:"user_id"`
	}
	form := url.Values{
		"client_id":     {c.cfg.AppID},
		"client_secret": {c.cfg.AppSecret},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {c.cfg.RedirectURI},
		"code":          {code},
	}
	if err := c.post(ctx, c.cfg.Base+"/oauth/access_token", form, &out); err != nil {
		return social.ThreadsToken{}, err
	}
	return social.ThreadsToken{AccessToken: out.AccessToken, UserID: out.UserID.String()}, nil
}

// LongLived trades a short-lived token for a 60-day one.
func (c *Client) LongLived(ctx context.Context, token string) (social.ThreadsToken, error) {
	return c.tokenGET(ctx, "/access_token", url.Values{
		"grant_type":    {"th_exchange_token"},
		"client_secret": {c.cfg.AppSecret},
		"access_token":  {token},
	})
}

// Refresh renews a long-lived token. Threads refuses a token younger than a day.
func (c *Client) Refresh(ctx context.Context, token string) (social.ThreadsToken, error) {
	return c.tokenGET(ctx, "/refresh_access_token", url.Values{
		"grant_type":   {"th_refresh_token"},
		"access_token": {token},
	})
}

func (c *Client) tokenGET(ctx context.Context, path string, q url.Values) (social.ThreadsToken, error) {
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := c.get(ctx, c.cfg.Base+path, q, &out); err != nil {
		return social.ThreadsToken{}, err
	}
	life := time.Duration(out.ExpiresIn) * time.Second
	if life <= 0 {
		life = defaultTokenLife
	}
	return social.ThreadsToken{AccessToken: out.AccessToken, ExpiresIn: life}, nil
}

// Me returns the id and username the token belongs to.
func (c *Client) Me(ctx context.Context, token string) (string, string, error) {
	var out struct {
		ID       json.Number `json:"id"`
		Username string      `json:"username"`
	}
	q := url.Values{"fields": {"id,username"}, "access_token": {token}}
	if err := c.get(ctx, c.cfg.Base+apiVersion+"/me", q, &out); err != nil {
		return "", "", err
	}
	return out.ID.String(), out.Username, nil
}

// KeywordSearch returns the public posts that match query, as the connected
// account. searchType is social.KeywordSearchTop or social.KeywordSearchRecent.
// While the app lacks the keyword permission, Meta answers with the account's
// own posts only.
func (c *Client) KeywordSearch(ctx context.Context, query, searchType string) ([]social.ThreadsKeywordPost, error) {
	conn, err := c.connection(ctx)
	if err != nil {
		return nil, err
	}
	var out struct {
		Data []struct {
			ID         string `json:"id"`
			Text       string `json:"text"`
			Permalink  string `json:"permalink"`
			Username   string `json:"username"`
			Timestamp  string `json:"timestamp"`
			HasReplies bool   `json:"has_replies"`
		} `json:"data"`
	}
	q := url.Values{
		"q":            {query},
		"search_type":  {searchType},
		"fields":       {"id,text,permalink,timestamp,username,has_replies"},
		"limit":        {strconv.Itoa(keywordSearchLimit)},
		"access_token": {conn.AccessToken},
	}
	if err := c.get(ctx, c.cfg.Base+apiVersion+"/keyword_search", q, &out); err != nil {
		return nil, err
	}
	posts := make([]social.ThreadsKeywordPost, 0, len(out.Data))
	for _, p := range out.Data {
		posts = append(posts, social.ThreadsKeywordPost{
			ID:         p.ID,
			Text:       p.Text,
			Permalink:  p.Permalink,
			Username:   p.Username,
			Timestamp:  parseGraphTime(p.Timestamp),
			HasReplies: p.HasReplies,
		})
	}
	return posts, nil
}

// parseGraphTime reads a Graph timestamp. A value in another shape becomes the
// zero time, which falls outside every window the digest keeps.
func parseGraphTime(s string) time.Time {
	t, err := time.Parse(graphTimeLayout, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// ThreadsPermalink returns the public URL of a published Threads post.
func (c *Client) ThreadsPermalink(ctx context.Context, mediaID string) (string, error) {
	conn, err := c.connection(ctx)
	if err != nil {
		return "", err
	}
	var out struct {
		Permalink string `json:"permalink"`
	}
	q := url.Values{"fields": {"permalink"}, "access_token": {conn.AccessToken}}
	if err := c.get(ctx, c.cfg.Base+apiVersion+"/"+url.PathEscape(mediaID), q, &out); err != nil {
		return "", err
	}
	return out.Permalink, nil
}

// PublishThreads publishes post to the connected account and returns the media
// id. Each container is polled until Threads reports it finished, and only then
// published. A carousel's children are finished before the carousel is made.
func (c *Client) PublishThreads(ctx context.Context, post social.Post) (string, error) {
	conn, err := c.connection(ctx)
	if err != nil {
		return "", err
	}
	creation, err := c.createPost(ctx, conn, post)
	if err != nil {
		return "", err
	}
	var out struct {
		ID string `json:"id"`
	}
	form := url.Values{"creation_id": {creation}, "access_token": {conn.AccessToken}}
	if err := c.post(ctx, c.userPath(conn)+"/threads_publish", form, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func (c *Client) createPost(ctx context.Context, conn social.ThreadsConnection, post social.Post) (string, error) {
	text := post.ThreadsText
	if text == "" {
		text = post.Caption
	}
	base := url.Values{"text": {text}}
	if post.ThreadsTopic != "" {
		base.Set("topic_tag", post.ThreadsTopic)
	}
	switch post.Kind {
	case social.TypeText:
		base.Set("media_type", "TEXT")
		return c.container(ctx, conn, base)
	case social.TypeImage:
		return c.container(ctx, conn, mediaForm(base, "IMAGE", post, post.Media[0].URL, false))
	case social.TypeReel:
		return c.container(ctx, conn, mediaForm(base, "VIDEO", post, post.Media[0].URL, false))
	case social.TypeStory:
		return "", fmt.Errorf("%w: stories go to instagram and facebook only", social.ErrPermanent)
	case social.TypeCarousel:
		children := make([]string, 0, len(post.Media))
		for _, m := range post.Media {
			id, err := c.container(ctx, conn, mediaForm(url.Values{}, "IMAGE", post, m.URL, true))
			if err != nil {
				return "", err
			}
			children = append(children, id)
		}
		base.Set("media_type", "CAROUSEL")
		base.Set("children", strings.Join(children, ","))
		return c.container(ctx, conn, base)
	default:
		return "", fmt.Errorf("%w: unknown post kind %q", social.ErrPermanent, post.Kind)
	}
}

// mediaForm adds one media item to form. A carousel child is flagged as such,
// and the post's alt text goes with every item that has media.
func mediaForm(form url.Values, mediaType string, post social.Post, mediaURL string, carouselItem bool) url.Values {
	form.Set("media_type", mediaType)
	if mediaType == "VIDEO" {
		form.Set("video_url", mediaURL)
	} else {
		form.Set("image_url", mediaURL)
	}
	if post.AltText != "" {
		form.Set("alt_text", post.AltText)
	}
	if carouselItem {
		form.Set("is_carousel_item", "true")
	}
	return form
}

// container creates one Threads container and waits until it is finished.
func (c *Client) container(ctx context.Context, conn social.ThreadsConnection, form url.Values) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	form = maps.Clone(form)
	form.Set("access_token", conn.AccessToken)
	if err := c.post(ctx, c.userPath(conn)+"/threads", form, &out); err != nil {
		return "", err
	}
	if err := c.await(ctx, conn, out.ID); err != nil {
		return "", err
	}
	return out.ID, nil
}

// await polls a container's status until Threads reports it finished. An error
// or expiry carries Threads' own message. A wait longer than PollFor fails the
// attempt, and the publisher retries it later.
func (c *Client) await(ctx context.Context, conn social.ThreadsConnection, id string) error {
	deadline := time.Now().Add(c.cfg.PollFor)
	for {
		var st struct {
			Status       string `json:"status"`
			ErrorMessage string `json:"error_message"`
		}
		q := url.Values{"fields": {"status,error_message"}, "access_token": {conn.AccessToken}}
		if err := c.get(ctx, c.cfg.Base+apiVersion+"/"+url.PathEscape(id), q, &st); err != nil {
			return err
		}
		switch st.Status {
		case "FINISHED":
			return nil
		case "ERROR", "EXPIRED":
			return fmt.Errorf("threads container %s ended as %s: %s", id, st.Status, st.ErrorMessage)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("threads container %s still processing after %s", id, c.cfg.PollFor)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for threads container: %w", ctx.Err())
		case <-time.After(c.cfg.PollEvery):
		}
	}
}

func (c *Client) userPath(conn social.ThreadsConnection) string {
	return c.cfg.Base + apiVersion + "/" + url.PathEscape(conn.UserID)
}

// connection reads the one connected account. Without one the post waits: it is
// retryable, and the owner is asked to reconnect by the maintenance email.
func (c *Client) connection(ctx context.Context) (social.ThreadsConnection, error) {
	conn, ok, err := c.store.LoadThreads(ctx)
	if err != nil {
		return social.ThreadsConnection{}, fmt.Errorf("threads: load connection: %w", err)
	}
	if !ok {
		return social.ThreadsConnection{}, errors.New("threads: the account is not connected")
	}
	return conn, nil
}

func (c *Client) get(ctx context.Context, rawURL string, params url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("threads: build request: %w", err)
	}
	return c.send(req, out)
}

func (c *Client) post(ctx context.Context, rawURL string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("threads: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.send(req, out)
}

// send is the one place a request leaves this package. It is guarded by the
// breaker, and any failure is returned with the URL and the token removed.
func (c *Client) send(req *http.Request, out any) error {
	if err := c.cb.AllowRequest(); err != nil {
		return fmt.Errorf("threads: %w", err)
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
		return fmt.Errorf("threads: read response: %w", err)
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
			return fmt.Errorf("threads: decode response: %w", err)
		}
	}
	return nil
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

// apiError turns a Graph error into an error. A client error other than 429 is
// permanent, except a token error, which waits for a reconnect.
func apiError(status int, raw []byte) error {
	var body graphErrorBody
	if err := json.Unmarshal(raw, &body); err != nil {
		// A body that is not Graph's error shape leaves only the status text.
		body = graphErrorBody{}
	}
	msg := body.Error.Message
	if msg == "" {
		msg = http.StatusText(status)
	}
	text := fmt.Sprintf("threads answered %s (code %s): %s", strconv.Itoa(status), strconv.Itoa(body.Error.Code), msg)
	if body.Error.Code != tokenErrorCode && status >= http.StatusBadRequest && status < http.StatusInternalServerError && status != http.StatusTooManyRequests {
		return fmt.Errorf("%w: %s", social.ErrPermanent, text)
	}
	return errors.New(text)
}

// redactTransport keeps the failure and drops the URL, which carries the token
// and the app secret as query parameters.
func redactTransport(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("threads request %s failed: %w", urlErr.Op, urlErr.Err)
	}
	return fmt.Errorf("threads request failed: %w", err)
}
