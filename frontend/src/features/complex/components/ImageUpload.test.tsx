import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ImageUpload } from './ImageUpload';
import type { Complex } from '@/shared/types/api.types';

vi.mock('../hooks/useUploadImage', () => ({
  useUploadImage: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
  useDeleteImage: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

const mockComplex: Complex = {
  id: 'c1',
  owner_id: 'u1',
  name: 'Padel Club',
  slug: 'padel-club',
  amenities: [],
  payments_enabled: false,
  address: 'Av. Libertador 1234',
  city: 'Buenos Aires',
  province: 'CABA',
  country_code: 'AR',
  currency: 'ARS',
  phone: '1155550000',
  email: null,
  logo_url: null,
  cover_url: null,
  deposit_percentage: 30,
  cancellation_hours: 24,
  latitude: null,
  longitude: null,
  is_active: true,
  created_at: '2026-01-10T10:00:00Z',
  updated_at: '2026-01-10T10:00:00Z',
  version: 1,
};

describe('ImageUpload', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // The two images ARE the buttons now. A separate "Subir logo" beside a logo
  // slot that already opens the file picker was the same action twice, and
  // there were two of them in two different places. What has to stay true is
  // that each image is reachable and says which one it is.
  it('gives each image its own labelled target', () => {
    render(<ImageUpload complex={mockComplex} />);
    expect(screen.getByRole('button', { name: 'Logo' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Portada' })).toBeInTheDocument();
  });

  // A preview is decoration in a settings panel: it must not compete with
  // the page's own data for the connection, and it must not resize the slot
  // when it arrives (PERF-08).
  it('defers each preview and declares the size of the box it fills', () => {
    render(<ImageUpload complex={{ ...mockComplex, logo_url: 'https://cdn.test/logo.webp' }} />);

    const logo = screen.getByAltText('Logo');
    expect(logo).toHaveAttribute('loading', 'lazy');
    expect(logo).toHaveAttribute('width', '512');
    expect(logo).toHaveAttribute('height', '512');
  });

  it('renders file format description', () => {
    render(<ImageUpload complex={mockComplex} />);
    expect(screen.getByText(/JPG, PNG o WebP/i)).toBeInTheDocument();
  });

  it('shows the uploaded image in its slot', () => {
    const complexWithLogo = { ...mockComplex, logo_url: 'https://test.com/logo.jpg' };
    render(<ImageUpload complex={complexWithLogo} />);
    expect(screen.getByRole('img', { name: 'Logo' })).toHaveAttribute('src', 'https://test.com/logo.jpg');
  });

  it('renders delete button when logo exists', () => {
    const complexWithLogo = { ...mockComplex, logo_url: 'https://test.com/logo.jpg' };
    render(<ImageUpload complex={complexWithLogo} />);
    // The delete button has a trash icon
    const deleteButtons = screen.getAllByRole('button');
    expect(deleteButtons.length).toBeGreaterThan(2);
  });
});
