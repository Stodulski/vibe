import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ComplexHeader } from './ComplexHeader';
import type { CourtWithPrices, PublicComplex, Schedule } from '../api/types';

const mockComplex: PublicComplex = {
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
  email: 'info@clubnorte.com',
  latitude: -34.5,
  longitude: -58.5,
  deposit_percentage: 30,
  cancellation_hours: 24,
  payments_enabled: true,
};

const mockSchedules: Schedule[] = [
  {
    id: 's1',
    complex_id: 'c1',
    day: 'monday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's2',
    complex_id: 'c1',
    day: 'tuesday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's3',
    complex_id: 'c1',
    day: 'wednesday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's4',
    complex_id: 'c1',
    day: 'thursday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's5',
    complex_id: 'c1',
    day: 'friday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's6',
    complex_id: 'c1',
    day: 'saturday',
    open_time: '09:00',
    close_time: '22:00',
    is_closed: false,
  },
  {
    id: 's7',
    complex_id: 'c1',
    day: 'sunday',
    open_time: '09:00',
    close_time: '20:00',
    is_closed: false,
  },
];

describe('ComplexHeader', () => {
  it('renders complex name', () => {
    render(<ComplexHeader complex={mockComplex} schedules={mockSchedules} />);
    expect(screen.getByRole('heading', { name: 'Club Padel Norte' })).toBeInTheDocument();
  });

  it('offers a share button for the club in its header', () => {
    render(<ComplexHeader complex={mockComplex} schedules={mockSchedules} />);
    expect(screen.getByRole('button', { name: 'Compartir Club Padel Norte' })).toBeInTheDocument();
  });

  it('renders address and city', () => {
    render(<ComplexHeader complex={mockComplex} schedules={mockSchedules} />);
    expect(screen.getByText(/av\. libertador 1234.*buenos aires/i)).toBeInTheDocument();
  });

  it('renders phone number', () => {
    render(<ComplexHeader complex={mockComplex} schedules={mockSchedules} />);
    expect(screen.getByText('+5491155550000')).toBeInTheDocument();
  });

  it('renders initial letter when no logo', () => {
    render(<ComplexHeader complex={mockComplex} schedules={mockSchedules} />);
    expect(screen.getByText('C')).toBeInTheDocument();
  });

  it('renders logo image when logo_url is present', () => {
    const complexWithLogo = {
      ...mockComplex,
      logo_url: 'https://example.com/logo.png',
    };
    render(<ComplexHeader complex={complexWithLogo} schedules={mockSchedules} />);
    const img = screen.getByAltText('Logo de Club Padel Norte');
    expect(img).toBeInTheDocument();
    expect(img).toHaveAttribute('src', 'https://example.com/logo.png');
  });

  it('does not render description when empty', () => {
    const noDesc = { ...mockComplex, description: '' };
    render(<ComplexHeader complex={noDesc} schedules={mockSchedules} />);
    expect(screen.queryByText('El mejor club de padel')).not.toBeInTheDocument();
  });

  it('does not render phone when not provided', () => {
    const noPhone = { ...mockComplex, phone: '' };
    render(<ComplexHeader complex={noPhone} schedules={mockSchedules} />);
    expect(screen.queryByText('+5491155550000')).not.toBeInTheDocument();
  });

});

describe('ComplexHeader cover alt text', () => {
  it('describes the cover photo by the club, its sports and its city', () => {
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
    const complexWithCover = { ...mockComplex, cover_url: 'https://example.com/cover.jpg' };
    render(<ComplexHeader complex={complexWithCover} schedules={mockSchedules} courts={[padelCourt]} />);
    const img = screen.getByAltText('Club Padel Norte, canchas de pádel en Buenos Aires');
    expect(img).toHaveAttribute('src', 'https://example.com/cover.jpg');
  });
});
