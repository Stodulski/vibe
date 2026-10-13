package meta

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/stodulski/vibe-server/internal/social"
)

// AspectProbe asks Instagram whether it accepts a 3:4 image. It creates one
// unpublished image container from the probe image and waits for Meta to finish
// or refuse it. It never publishes, and Instagram discards the container itself
// after 24 hours.
type AspectProbe struct {
	client   *Client
	imageURL string
	logger   *slog.Logger
}

// NewAspectProbe returns a probe that fetches imageURL.
func NewAspectProbe(client *Client, imageURL string, logger *slog.Logger) *AspectProbe {
	return &AspectProbe{client: client, imageURL: imageURL, logger: logger}
}

// Check is the probe's line of the connection email. A refusal is information,
// not a problem: the publisher does not depend on the answer.
func (p *AspectProbe) Check(ctx context.Context, report social.ConnectionReport) social.ReportLine {
	const label = "Formato 3:4: "
	switch {
	case p.imageURL == "":
		return social.ReportLine{Text: label + "no probado, no hay URL pública para la imagen de prueba."}
	case report.IGUserID == "":
		return social.ReportLine{Text: label + "no probado, falta la cuenta de Instagram."}
	}

	err := p.probe(ctx, report.IGUserID)
	if err != nil {
		p.logger.Warn("social: 3:4 probe refused", "error", err)
		return social.ReportLine{Text: label + "rechazado: " + err.Error() + "."}
	}
	p.logger.Info("social: 3:4 probe accepted")
	return social.ReportLine{Text: label + "aceptado."}
}

// probe creates the container and polls it until it is finished, refused or the
// wait runs out. Only Instagram's own refusal text is returned as the reason.
func (p *AspectProbe) probe(ctx context.Context, igUserID string) error {
	id, err := p.client.createContainer(ctx, igUserID, url.Values{"image_url": {p.imageURL}})
	if err != nil {
		return err
	}
	deadline := time.Now().Add(p.client.cfg.PollFor)
	for {
		var st struct {
			StatusCode string `json:"status_code"`
			Status     string `json:"status"`
		}
		if err := p.client.get(ctx, p.client.cfg.Token, "/"+id, url.Values{"fields": {"status_code,status"}}, &st); err != nil {
			return err
		}
		switch st.StatusCode {
		case "FINISHED":
			return nil
		case "ERROR", "EXPIRED":
			if st.Status != "" {
				return errors.New(st.Status)
			}
			return fmt.Errorf("the container ended as %s", st.StatusCode)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("still processing after %s", p.client.cfg.PollFor)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for the probe: %w", ctx.Err())
		case <-time.After(p.client.cfg.PollEvery):
		}
	}
}
