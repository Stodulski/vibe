import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ComplexMap } from './ComplexMap';

// Mirrors how react-leaflet actually behaves, which is the whole point of
// this mock: the container element exists on the first render, but the
// Leaflet map instance is built in the child's own effect and only handed to
// the forwarded ref afterwards. A mock that resolved the ref synchronously
// hid a real bug — the effect that names the map ran once, against nothing.
vi.mock('react-leaflet', async () => {
  const { useEffect, useState } = await import('react');
  return {
    MapContainer: ({
      children,
      className,
      ref,
    }: {
      children: React.ReactNode;
      className?: string;
      ref?: (map: unknown) => void;
    }) => {
      const [el, setEl] = useState<HTMLDivElement | null>(null);

      useEffect(() => {
        if (!el || !ref) return;
        ref({ getContainer: () => el, invalidateSize: () => undefined });
        return () => {
          ref(null);
        };
      }, [el, ref]);

      return (
        <div data-testid="map-container" className={className} ref={setEl}>
          {children}
        </div>
      );
    },
    TileLayer: () => <div data-testid="tile-layer" />,
    Marker: ({ children }: { children?: React.ReactNode }) => <div data-testid="map-marker">{children}</div>,
    Popup: ({ children }: { children?: React.ReactNode }) => <div data-testid="map-popup">{children}</div>,
  };
});

// Recorded at import time: `markerIcon` is built once, at module scope.
const { iconCalls } = vi.hoisted(() => ({ iconCalls: [] as Record<string, unknown>[] }));

vi.mock('leaflet', () => ({
  icon: (options: Record<string, unknown>) => {
    iconCalls.push(options);
    return {};
  },
}));

const defaultProps = {
  latitude: -34.5875,
  longitude: -58.3974,
  name: 'Padel Club Norte',
  address: 'Av. Libertador 1234, Buenos Aires',
};

// Split from the rendering cases below only to stay under the repo's
// max-lines-per-function cap; they share `defaultProps` and the mocks above.
describe('ComplexMap marker and accessible name', () => {
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

  // The map instance arrives a render after the element does, so this only
  // holds if the component reacts to it landing rather than reading a ref
  // once on mount.
  it('gives the map an accessible name once Leaflet hands over the instance', async () => {
    render(<ComplexMap {...defaultProps} />);

    const map = screen.getByTestId('map-container');
    await waitFor(() => {
      expect(map).toHaveAttribute('role', 'application');
    });
    expect(map).toHaveAccessibleName('Mapa de Padel Club Norte, Av. Libertador 1234, Buenos Aires');
  });

  it('renames the map when the club it shows changes', async () => {
    const { rerender } = render(<ComplexMap {...defaultProps} />);
    const map = screen.getByTestId('map-container');
    await waitFor(() => {
      expect(map).toHaveAccessibleName('Mapa de Padel Club Norte, Av. Libertador 1234, Buenos Aires');
    });

    rerender(<ComplexMap {...defaultProps} name="Padel Sur" address="Calle Falsa 123" />);

    await waitFor(() => {
      expect(map).toHaveAccessibleName('Mapa de Padel Sur, Calle Falsa 123');
    });
  });
});

describe('ComplexMap rendering', () => {
  beforeEach(() => {
    vi.clearAllMocks();
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
