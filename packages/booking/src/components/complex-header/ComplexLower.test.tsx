import { afterAll, beforeAll, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { renderToStaticMarkup } from 'react-dom/server';
import { ComplexLower } from './ComplexLower';
import { BOOKING_MESSAGES } from '../../messages';
import type { PublicComplex } from '../../api/types';

vi.mock('../ComplexMap', () => ({
  ComplexMap: () => <div data-testid="mock-map">Map</div>,
}));

// The map-failure test below makes Leaflet throw on purpose; keep its expected
// console noise out of the run, as the ErrorBoundary tests do.
const originalError = console.error;
beforeAll(() => {
  console.error = (...args: unknown[]) => {
    if (typeof args[0] === 'string' && args[0].includes('Leaflet failed to load')) {
      return;
    }
    originalError.call(console, ...args);
  };
});
afterAll(() => {
  console.error = originalError;
});

function makeComplex(overrides: Partial<PublicComplex> = {}): PublicComplex {
  return {
    id: 'c1',
    name: 'Club Padel Norte',
    slug: 'club-padel-norte',
    amenities: [],
    address: 'Av. Libertador 1234',
    city: 'Buenos Aires',
    province: 'CABA',
    country_code: 'AR',
    currency: 'ARS',
    phone: '+5491155550000',
    deposit_percentage: 30,
    cancellation_hours: 24,
    payments_enabled: true,
    ...overrides,
  };
}

describe('ComplexLower services', () => {
  it('lists the services under a heading for screen readers', () => {
    render(<ComplexLower complex={makeComplex({ amenities: ['parking', 'wifi'] })} />);

    expect(screen.getByRole('heading', { level: 2, name: BOOKING_MESSAGES.complex.amenitiesSection })).toBeInTheDocument();
    expect(screen.getByText(BOOKING_MESSAGES.complex.amenities.parking)).toBeInTheDocument();
    expect(screen.getByText(BOOKING_MESSAGES.complex.amenities.wifi)).toBeInTheDocument();
  });

  it('renders no services section when the club lists none', () => {
    render(<ComplexLower complex={makeComplex({ amenities: [] })} />);

    expect(screen.queryByRole('heading', { name: BOOKING_MESSAGES.complex.amenitiesSection })).not.toBeInTheDocument();
  });
});

describe('ComplexLower in the server markup', () => {
  // Crawlers and no-JS readers get the server HTML only, which is the stacked
  // layout. The services must be in it, outside any collapsed panel.
  it('writes every service into the HTML, not only inside a collapsed panel', () => {
    const html = renderToStaticMarkup(<ComplexLower complex={makeComplex({ amenities: ['parking', 'wifi'] })} />);

    expect(html).toContain(BOOKING_MESSAGES.complex.amenities.parking);
    expect(html).toContain(BOOKING_MESSAGES.complex.amenities.wifi);
  });

  it('lists each service once', () => {
    const html = renderToStaticMarkup(<ComplexLower complex={makeComplex({ amenities: ['parking', 'wifi'] })} />);

    expect(html.split(BOOKING_MESSAGES.complex.amenities.parking)).toHaveLength(2);
    expect(html.split(BOOKING_MESSAGES.complex.amenities.wifi)).toHaveLength(2);
  });

  // Below lg the page's flex column moves this block after the booking; at lg it
  // sits in the card, so its own top margin goes.
  it('places itself after the booking below lg and inside the card at lg', () => {
    const html = renderToStaticMarkup(<ComplexLower complex={makeComplex({ amenities: ['parking'] })} />);

    expect(html).toContain('class="mt-6 max-lg:order-1 sm:mt-10 lg:mt-0"');
  });
});

describe('ComplexLower map', () => {
  it('shows the map once the club has coordinates', async () => {
    render(<ComplexLower complex={makeComplex({ latitude: -34.5, longitude: -58.5 })} />);

    expect(await screen.findByTestId('mock-map')).toBeInTheDocument();
  });

  it('renders nothing when the club has neither services nor coordinates', () => {
    const { container } = render(<ComplexLower complex={makeComplex({ amenities: [] })} />);

    expect(container).toBeEmptyDOMElement();
  });

  it('shows a fallback instead of crashing the rest of the page when the map fails to render', async () => {
    vi.doMock('../ComplexMap', () => ({
      ComplexMap: () => {
        throw new Error('Leaflet failed to load');
      },
    }));
    vi.resetModules();
    const { ComplexLower: ComplexLowerWithBrokenMap } = await import('./ComplexLower');

    render(
      <ComplexLowerWithBrokenMap complex={makeComplex({ amenities: ['parking'], latitude: -34.5, longitude: -58.5 })} />,
    );

    expect(await screen.findByText('No pudimos cargar el mapa')).toBeInTheDocument();
    // The services sit outside the map's own boundary and are unaffected.
    expect(screen.getByText(BOOKING_MESSAGES.complex.amenities.parking)).toBeInTheDocument();
  });
});
