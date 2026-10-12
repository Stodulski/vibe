package meta

import (
	"context"
	"fmt"
	"net/url"

	"github.com/stodulski/vibe-server/internal/social"
)

// CheckConnection reads the Page and Instagram account the token reaches and
// the permissions it holds. It does not fail: every problem becomes an entry in
// the report, so the operator gets one email either way. It uses only the
// system-user token, the one thing the service has, because a debug_token call
// would need the app secret.
func (c *Client) CheckConnection(ctx context.Context) social.ConnectionReport {
	var report social.ConnectionReport

	pages, err := c.connectionPages(ctx)
	if err != nil {
		report.Problems = append(report.Problems, "no se pudieron leer las páginas: "+err.Error())
	} else {
		c.describeConnectionPage(pages, &report)
	}

	granted, err := c.grantedPermissions(ctx)
	if err != nil {
		report.Problems = append(report.Problems, "no se pudieron leer los permisos: "+err.Error())
	} else {
		report.Missing = missingPermissions(granted)
	}
	return report
}

type connectionPage struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	InstagramBusinessAccount *struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"instagram_business_account"`
}

func (c *Client) connectionPages(ctx context.Context) ([]connectionPage, error) {
	var out struct {
		Data []connectionPage `json:"data"`
	}
	fields := url.Values{"fields": {"id,name,instagram_business_account{id,username}"}}
	if err := c.get(ctx, c.cfg.Token, "/me/accounts", fields, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// describeConnectionPage fills the page and Instagram fields of the report, or
// records why it could not choose one.
func (c *Client) describeConnectionPage(pages []connectionPage, r *social.ConnectionReport) {
	selected := pages
	if c.cfg.PageID != "" {
		selected = filterConnectionPages(pages, c.cfg.PageID)
	}
	switch {
	case len(selected) == 0:
		r.Problems = append(r.Problems, "el token no ve ninguna página para publicar")
		return
	case len(selected) > 1:
		r.Problems = append(r.Problems, fmt.Sprintf("el token ve %d páginas; configurá META_SOCIAL_PAGE_ID", len(selected)))
		return
	}

	page := selected[0]
	r.PageID = page.ID
	r.PageName = page.Name
	if page.InstagramBusinessAccount == nil || page.InstagramBusinessAccount.ID == "" {
		r.Problems = append(r.Problems, "la página no tiene cuenta profesional de Instagram vinculada")
		return
	}
	r.IGUserID = page.InstagramBusinessAccount.ID
	r.IGUsername = page.InstagramBusinessAccount.Username
}

func filterConnectionPages(pages []connectionPage, id string) []connectionPage {
	var out []connectionPage
	for _, p := range pages {
		if p.ID == id {
			out = append(out, p)
		}
	}
	return out
}

func (c *Client) grantedPermissions(ctx context.Context) (map[string]bool, error) {
	var out struct {
		Data []struct {
			Permission string `json:"permission"`
			Status     string `json:"status"`
		} `json:"data"`
	}
	if err := c.get(ctx, c.cfg.Token, "/me/permissions", nil, &out); err != nil {
		return nil, err
	}
	granted := make(map[string]bool, len(out.Data))
	for _, p := range out.Data {
		granted[p.Permission] = p.Status == "granted"
	}
	return granted, nil
}

func missingPermissions(granted map[string]bool) []string {
	var missing []string
	for _, p := range social.RequiredPermissions {
		if !granted[p] {
			missing = append(missing, p)
		}
	}
	return missing
}
