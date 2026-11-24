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
  });});

describe('ComplexLower map in the server markup', () => {
  // The map itself is client-only, but the Google Maps link and the 200px
  // block that holds the map's height are plain markup: they must be in the
  // HTML, or the block grows by ~38px once the client mounts (CLS at lg).
  it('writes the Google Maps link for the club coordinates into the HTML', () => {
    const html = renderToStaticMarkup(<ComplexLower complex={makeComplex({ latitude: -34.5, longitude: -58.5 })} />);

    expect(html).toContain('href="https://www.google.com/maps/search/?api=1&amp;query=-34.5,-58.5"');
  });});

describe('ComplexLower map', () => {
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

describe('ComplexLower description', () => {
  it("shows the owner's description of the club, line breaks kept, before the services", () => {
    render(
      <ComplexLower
        complex={makeComplex({ description: 'Cuatro canchas de blindex.\nBar con parrilla.', amenities: ['parking'] })}
      />,
    );

    const about = screen.getByRole('region', { name: BOOKING_MESSAGES.complex.aboutSection });
    expect(about).toHaveTextContent('Cuatro canchas de blindex. Bar con parrilla.');
    expect(about.querySelector('p')?.textContent).toBe('Cuatro canchas de blindex.\nBar con parrilla.');
    const services = screen.getByRole('heading', { name: BOOKING_MESSAGES.complex.amenitiesSection });
    expect(about.compareDocumentPosition(services) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('shows the description even when the club lists no services and no map, and nothing for a blank one', () => {
    const { container, rerender } = render(<ComplexLower complex={makeComplex({ description: 'Pádel en Rosario.' })} />);
    expect(screen.getByText('Pádel en Rosario.')).toBeInTheDocument();

    rerender(<ComplexLower complex={makeComplex({ description: '   ' })} />);
    expect(container).toBeEmptyDOMElement();
  });
});
