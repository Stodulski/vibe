package instagram

import (
	"context"
	"errors"
	"fmt"
	"net/url"
)

// Publishing a story on graph.instagram.com, per
// https://developers.facebook.com/docs/instagram-platform/content-publishing
// (checked 2026-11-19):
//
//	POST /<IG_ID>/media                       media_type=STORIES, image_url -> container id
//	GET  /<CONTAINER_ID>?fields=status_code   IN_PROGRESS, FINISHED, ERROR, EXPIRED, PUBLISHED
//	POST /<IG_ID>/media_publish               creation_id -> media id
//	GET  /<IG_ID>/content_publishing_limit    quota_usage, config.quota_total
//
// Every method takes the account's id and token; the token travels in the
// POST body or the query string and never in an error.

// Container statuses Meta reports.
const (
	ContainerFinished   = "FINISHED"
	ContainerInProgress = "IN_PROGRESS"
	ContainerError      = "ERROR"
	ContainerExpired    = "EXPIRED"
	ContainerPublished  = "PUBLISHED"
)

// defaultPublishingLimit is what Meta documents (100 posts per 24 hours), used
// when the answer leaves quota_total out.
const defaultPublishingLimit = 100

// CreateStory creates a STORIES media container for an image Meta fetches from
// imageURL, and returns its id.
func (c *Client) CreateStory(ctx context.Context, igUserID, token, imageURL string) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	form := url.Values{"media_type": {"STORIES"}, "image_url": {imageURL}, "access_token": {token}}
	if err := c.post(ctx, c.cfg.GraphBase+"/"+url.PathEscape(igUserID)+"/media", form, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", fmt.Errorf("%w: the container answer carried no id", ErrUnavailable)
	}
	return out.ID, nil
}

// ContainerStatus reads a container's status_code.
func (c *Client) ContainerStatus(ctx context.Context, containerID, token string) (string, error) {
	var out struct {
		StatusCode string `json:"status_code"`
	}
	q := url.Values{"fields": {"status_code"}, "access_token": {token}}
	if err := c.get(ctx, c.cfg.GraphBase+"/"+url.PathEscape(containerID), q, &out); err != nil {
		return "", err
	}
	return out.StatusCode, nil
}

// PublishContainer publishes a FINISHED container and returns the media id.
func (c *Client) PublishContainer(ctx context.Context, igUserID, token, containerID string) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	form := url.Values{"creation_id": {containerID}, "access_token": {token}}
	if err := c.post(ctx, c.cfg.GraphBase+"/"+url.PathEscape(igUserID)+"/media_publish", form, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", fmt.Errorf("%w: the publish answer carried no media id", ErrUnavailable)
	}
	return out.ID, nil
}

// PublishingLimit returns how many posts the account published through the
// API in the last 24 hours and how many it may.
func (c *Client) PublishingLimit(ctx context.Context, igUserID, token string) (used, total int, err error) {
	var out struct {
		Data []struct {
			QuotaUsage int `json:"quota_usage"`
			Config     struct {
				QuotaTotal int `json:"quota_total"`
			} `json:"config"`
		} `json:"data"`
	}
	q := url.Values{"fields": {"quota_usage,config"}, "access_token": {token}}
	if err := c.get(ctx, c.cfg.GraphBase+"/"+url.PathEscape(igUserID)+"/content_publishing_limit", q, &out); err != nil {
		return 0, 0, err
	}
	if len(out.Data) == 0 {
		return 0, 0, errors.Join(ErrUnavailable, errors.New("instagram: content_publishing_limit answered no quota"))
	}
	total = out.Data[0].Config.QuotaTotal
	if total <= 0 {
		total = defaultPublishingLimit
	}
	return out.Data[0].QuotaUsage, total, nil
}
