import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ComplexMap } from './ComplexMap';

// Mock Leaflet components since they require DOM APIs not available in the test DOM
vi.mock('react-leaflet', () => ({
  MapContainer: ({ children, className }: { children: React.ReactNode; className?: string }) => (
    <div data-testid="map-container" className={className}>
      {children}
    </div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: ({ children }: { children?: React.ReactNode }) => <div data-testid="map-marker">{children}</div>,
  Popup: ({ children }: { children?: React.ReactNode }) => <div data-testid="map-popup">{children}</div>,
}));

vi.mock('leaflet', () => ({
  icon: () => ({}),
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
