// @vitest-environment node
//
// The entries are rendered on the server by the landing (Astro SSR). This file
// runs under the node environment, where there is no `window` and no `document`,
// so any browser-only access made while rendering throws here first.
import { describe, expect, it, vi } from 'vitest';
import { renderToString } from 'react-dom/server';
import { useQueryParams } from '../lib/navigation';
import { BookingRoot } from '../BookingRoot';
import type { BookingConfig } from '../config';
import type { CourtWithPrices, PublicComplexResponse } from '../api/types';
import { ComplexPageEntry } from './complex-page';
import { BookCancelEntry } from './book-cancel';
import { BookConfirmEntry } from './book-confirm';
import { BookSuccessEntry } from './book-success';
import { PaymentFailedEntry } from './payment-failed';

// Leaflet touches `window` when it is imported. The map must never be imported
// while the page renders on the server, so importing it here fails the test.
vi.mock('leaflet', () => {
  throw new Error('leaflet must not load during server render');
});

const config: BookingConfig = {
  apiBaseUrl: 'https://api.example.test/api/v1',
  publicSiteUrl: 'https://vibe.com.ar',
  privacyUrl: 'https://vibe.com.ar/privacidad',
};

const complexResponse: PublicComplexResponse = {
  complex: {
    id: 'c1',
    name: 'Club Ejemplo',
    slug: 'club-ejemplo',
    address: 'Av. Siempre Viva 742',
    city: 'Banfield',
    province: 'Buenos Aires',
    country_code: 'AR',
    currency: 'ARS',
    phone: '+5491100000000',
    deposit_percentage: 0,
    cancellation_hours: 24,
    latitude: -34.6,
    longitude: -58.4,
    amenities: [],
    payments_enabled: true,
  },
  courts: [],
  schedules: [],
};

describe('useQueryParams during server render', () => {
  it('reads the query the host passed in, not window.location', () => {
    function Probe() {
      const [params] = useQueryParams();
      return <span data-sport={params.get('sport') ?? 'none'}>probe</span>;
    }

    const html = renderToString(
      <BookingRoot config={config} initialSearch="?sport=padel&duration=90">
        <Probe />
      </BookingRoot>,
    );

    expect(html).toContain('data-sport="padel"');
  });

  it('reads an empty query when the host passes none', () => {
    function Probe() {
      const [params] = useQueryParams();
      return <span data-sport={params.get('sport') ?? 'none'}>probe</span>;
    }

    const html = renderToString(
      <BookingRoot config={config}>
        <Probe />
      </BookingRoot>,
    );

    expect(html).toContain('data-sport="none"');
  });
});

describe('entries render on the server', () => {
  it('renders the complex page with the complex it was given', () => {
    const html = renderToString(
      <ComplexPageEntry
        slug="club-ejemplo"
        config={config}
        initialComplex={{ slug: 'club-ejemplo', data: complexResponse }}
        initialSearch="?date=2026-10-20"
        toaster={false}
      />,
    );

    expect(html).toContain('Club Ejemplo');
  });

  it('renders the complex page without seed data (loading state)', () => {
    expect(() =>
      renderToString(<ComplexPageEntry slug="club-ejemplo" config={config} toaster={false} />),
    ).not.toThrow();
  });

  it('keeps the map out of the server markup and out of the module graph', () => {
    const html = renderToString(
      <ComplexPageEntry
        slug="club-ejemplo"
        config={config}
        initialComplex={{ slug: 'club-ejemplo', data: complexResponse }}
        toaster={false}
      />,
    );

    expect(html).not.toContain('leaflet');
  });

  it('renders the cancel page for a token', () => {
    expect(() =>
      renderToString(
        <BookCancelEntry slug="club-ejemplo" config={config} initialSearch="?token=x" toaster={false} />,
      ),
    ).not.toThrow();
  });

  it('renders the payment-failed route without touching storage', () => {
    expect(() =>
      renderToString(
        <PaymentFailedEntry slug="club-ejemplo" config={config} initialSearch="?error=payment_failed" toaster={false} />,
      ),
    ).not.toThrow();
  });

  it('renders the confirm and success entries without a window', () => {
    expect(() =>
      renderToString(<BookConfirmEntry slug="club-ejemplo" config={config} toaster={false} />),
    ).not.toThrow();
    expect(() =>
      renderToString(<BookSuccessEntry slug="club-ejemplo" config={config} initialSearch="?status=pending" toaster={false} />),
    ).not.toThrow();
  });
});

describe('the share button on the server', () => {
  it('renders the share button of the complex page', () => {
    const html = renderToString(
      <ComplexPageEntry
        slug="club-ejemplo"
        config={config}
        initialComplex={{ slug: 'club-ejemplo', data: complexResponse }}
        toaster={false}
      />,
    );

    expect(html).toContain('aria-label="Compartir Club Ejemplo"');
  });
});

describe('complex images on the server', () => {
  const padelCourt: CourtWithPrices = {
    id: 'k1',
    complex_id: 'c1',
    name: 'Cancha 1',
    sport: 'padel',
    court_type: 'indoor',
    is_active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices: [],
  };

  it('renders the cover and the logo as <img> tags with descriptive alt text', () => {
    const html = renderToString(
      <ComplexPageEntry
        slug="club-ejemplo"
        config={config}
        initialComplex={{
          slug: 'club-ejemplo',
          data: {
            ...complexResponse,
            complex: {
              ...complexResponse.complex,
              cover_url: 'https://img.example.test/cover.jpg',
              logo_url: 'https://img.example.test/logo.png',
            },
            courts: [padelCourt],
          },
        }}
        toaster={false}
      />,
    );

    // HTML attribute names are case-insensitive: React may emit fetchPriority, browsers read fetchpriority.
    expect(html).toMatch(
      /<img[^>]*src="https:\/\/img\.example\.test\/cover\.jpg"[^>]*alt="Club Ejemplo, canchas de pádel en Banfield"[^>]*fetchpriority="high"/i,
    );
    expect(html).toMatch(/<img[^>]*src="https:\/\/img\.example\.test\/logo\.png"[^>]*alt="Logo de Club Ejemplo"/);
  });
});
