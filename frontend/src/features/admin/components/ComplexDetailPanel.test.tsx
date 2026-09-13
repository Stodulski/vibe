import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ComplexDetailPanel } from './ComplexDetailPanel';
import type { AdminComplexDetailResponse } from '@/shared/types/api.types';

const t = ES_AR;

const mockData: AdminComplexDetailResponse = {
  complex: {
    id: 'c1',
    owner_id: 'u1',
    name: 'Padel Club Norte',
    slug: 'padel-club-norte',
    amenities: [],
    payments_enabled: false,
    address: 'Av. Libertador 1234',
    city: 'Buenos Aires',
    province: 'CABA',
    country_code: 'AR',
    currency: 'ARS',
    phone: '1155550000',
    email: 'info@padelclub.com',
    logo_url: null,
    cover_url: null,
    deposit_percentage: 30,
    cancellation_hours: 24,
    latitude: -34.5,
    longitude: -58.5,
    is_active: true,
    created_at: '2026-01-10T10:00:00Z',
    updated_at: '2026-01-10T10:00:00Z',
    version: 1,
  },
  owner_name: 'Juan Perez',
  owner_email: 'juan@test.com',
  courts_count: 4,
  clients_count: 150,
  bookings_count: 1200,
  total_revenue: 1800000000,
};

function renderPanel(data: AdminComplexDetailResponse = mockData) {
  return renderWithProviders(<ComplexDetailPanel data={data} />, {
    initialEntries: ['/admin/complexes/c1'],
  });
}

describe('ComplexDetailPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders complex name', () => {
    renderPanel();
    expect(screen.getByText('Padel Club Norte')).toBeInTheDocument();
  });

  it('renders complex city and province', () => {
    renderPanel();
    expect(screen.getByText('Buenos Aires, CABA')).toBeInTheDocument();
  });

  it('renders complex slug', () => {
    renderPanel();
    expect(screen.getByText('/padel-club-norte')).toBeInTheDocument();
  });

  it('renders courts count stat', () => {
    renderPanel();
    expect(screen.getByText('4')).toBeInTheDocument();
  });

  it('renders clients count stat', () => {
    renderPanel();
    expect(screen.getByText('150')).toBeInTheDocument();
  });

  it('renders bookings count stat', () => {
    renderPanel();
    expect(screen.getByText((content) => content.includes('1.200'))).toBeInTheDocument();
  });

  it('renders owner name', () => {
    renderPanel();
    expect(screen.getByText('Juan Perez')).toBeInTheDocument();
  });

  it('renders owner email', () => {
    renderPanel();
    expect(screen.getByText('juan@test.com')).toBeInTheDocument();
  });

  it('renders active badge when complex is active', () => {
    renderPanel();
    expect(screen.getByText(/activo/i)).toBeInTheDocument();
  });

  it('renders inactive badge when complex is inactive', () => {
    const inactiveData = {
      ...mockData,
      complex: { ...mockData.complex, is_active: false },
    };
    renderPanel(inactiveData);
    expect(screen.getByText(/inactivo/i)).toBeInTheDocument();
  });

  // Finding M12: this was a `<button onClick={() => navigate(...)}>`, so
  // there was no href to hover or open in a new tab. It is now a real link.
  it('links back to the complexes list', () => {
    renderPanel();
    const link = screen.getByRole('link', { name: t.admin.detail.backToComplexes });
    expect(link).toHaveAttribute('href', '/admin/complexes');
  });

  // Finding M12: the owner link used to be a `<button>` labelled
  // "Volver a usuarios" ("Back to users") while it actually navigated to the
  // owner's own detail page — a copy bug from reusing the back-button label.
  it('links to the owner detail page with copy describing what it does', () => {
    renderPanel();
    const link = screen.getByRole('link', { name: t.admin.detail.viewOwnerProfile });
    expect(link).toHaveAttribute('href', '/admin/users/u1');
    expect(screen.queryByText(t.admin.detail.backToUsers)).not.toBeInTheDocument();
  });
});
