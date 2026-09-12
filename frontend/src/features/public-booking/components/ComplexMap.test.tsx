import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ComplexMap } from './ComplexMap';

// Mock Leaflet components since they require DOM APIs not available in the test DOM
vi.mock('react-leaflet', () => ({
  // Hands the component the same thing Leaflet does: an object whose
  // `getContainer()` is the element actually rendered, so the effect that
  // names the map has something real to write on.
  MapContainer: ({
    children,
    className,
    ref,
  }: {
    children: React.ReactNode;
    className?: string;
    ref?: { current: unknown };
  }) => (
    <div
      data-testid="map-container"
      className={className}
      ref={(el) => {
        if (el && ref) ref.current = { getContainer: () => el, invalidateSize: () => undefined };
      }}
    >
      {children}
    </div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: ({ children }: { children?: React.ReactNode }) => <div data-testid="map-marker">{children}</div>,
  Popup: ({ children }: { children?: React.ReactNode }) => <div data-testid="map-popup">{children}</div>,
}));

// Recorded at import time: `markerIcon` is built once, at module scope.
const { iconCalls } = vi.hoisted(() => ({ iconCalls: [] as Record<string, unknown>[] }));

vi.mock('leaflet', () => ({
  icon: (options: Record<string, unknown>) => {
    iconCalls.push(options);
    return {};
  },
}));

describe('ComplexMap', () => {
  const defaultProps = {
    latitude: -34.5875,
    longitude: -58.3974,
    name: 'Padel Club Norte',
    address: 'Av. Libertador 1234, Buenos Aires',
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  // The three marker images used to be fetched from unpkg.com at runtime:
  // a third-party CDN in the storefront's render path, dead offline and
  // under a strict CSP (MAP-03). They are bundled now, so every URL the
  // icon gets is same-origin (a hashed asset path, or a data: URI for the
  // ones Vite inlines).
  it('builds its marker from bundled assets instead of a third-party CDN', () => {
    const options = iconCalls[0];
    expect(options).toBeDefined();

    for (const key of ['iconUrl', 'iconRetinaUrl', 'shadowUrl']) {
      const url = options?.[key];
      expect(typeof url).toBe('string');
      expect(url as string).not.toMatch(/^https?:\/\//);
    }
  });

  it('gives the map an accessible name instead of an anonymous tile box', () => {
    render(<ComplexMap {...defaultProps} />);

    const map = screen.getByTestId('map-container');
    expect(map).toHaveAttribute('role', 'application');
    expect(map).toHaveAccessibleName('Mapa de Padel Club Norte, Av. Libertador 1234, Buenos Aires');
  });

  it('renders map container', () => {
    render(<ComplexMap {...defaultProps} />);
    expect(screen.getByTestId('map-container')).toBeInTheDocument();
  });

  it('renders map marker', () => {
    render(<ComplexMap {...defaultProps} />);
    expect(screen.getByTestId('map-marker')).toBeInTheDocument();
  });

  it('renders complex name in popup', () => {
    render(<ComplexMap {...defaultProps} />);
    expect(screen.getByText('Padel Club Norte')).toBeInTheDocument();
  });

  it('renders address in popup', () => {
    render(<ComplexMap {...defaultProps} />);
    expect(screen.getByText('Av. Libertador 1234, Buenos Aires')).toBeInTheDocument();
  });

  it('renders Google Maps link', () => {
    render(<ComplexMap {...defaultProps} />);
    const link = screen.getByRole('link', { name: /google maps/i });
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute('href', expect.stringContaining('google.com/maps'));
    expect(link).toHaveAttribute('target', '_blank');
  });

  it('includes coordinates in Google Maps link', () => {
    render(<ComplexMap {...defaultProps} />);
    const link = screen.getByRole('link', { name: /google maps/i });
    expect(link).toHaveAttribute('href', expect.stringContaining('-34.5875'));
    expect(link).toHaveAttribute('href', expect.stringContaining('-58.3974'));
  });
});
