import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { UserDetailPanel } from './UserDetailPanel';
import type { User, Complex } from '@/shared/types/api.types';

const t = ES_AR;

const mockToggleMutate = vi.fn();

vi.mock('../hooks/useToggleUserActive', () => ({
  useToggleUserActive: () => ({
    mutate: mockToggleMutate,
    isPending: false,
  }),
}));

// DATA-11: who is signed in comes from `useAuth`'s query cache, not the store.
vi.mock('@/features/auth', () => ({
  useAuth: () => ({
    user: { id: 'current-user', role: 'superadmin' },
    isLoading: false,
    isAuthenticated: true,
  }),
}));

const mockUser: User = {
  id: 'u1',
  email: 'juan@test.com',
  first_name: 'Juan',
  last_name: 'Perez',
  role: 'owner',
  phone: '1155550000',
  is_active: true,
  email_verified: true,
  created_at: '2026-01-15T10:00:00Z',
  updated_at: '2026-01-15T10:00:00Z',
};

const mockComplexes: Complex[] = [
  {
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
    email: null,
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
];

function renderPanel(user: User = mockUser, complexes: Complex[] = mockComplexes) {
  return renderWithProviders(<UserDetailPanel user={user} complexes={complexes} />, {
    initialEntries: [`/admin/users/${user.id}`],
  });
}

describe('UserDetailPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders user full name', () => {
    renderPanel();
    expect(screen.getByText('Juan Perez')).toBeInTheDocument();
  });

  it('renders user email', () => {
    renderPanel();
    expect(screen.getByText('juan@test.com')).toBeInTheDocument();
  });

  it('renders user phone', () => {
    renderPanel();
    expect(screen.getByText('1155550000')).toBeInTheDocument();
  });

  it('renders complexes count', () => {
    renderPanel();
    expect(screen.getByText('1 complejo')).toBeInTheDocument();
  });

  it('renders complex name in complexes section', () => {
    renderPanel();
    expect(screen.getByText('Padel Club Norte')).toBeInTheDocument();
  });

  it('renders deactivate button for non-self user', () => {
    renderPanel();
    expect(screen.getByRole('button', { name: /desactivar/i })).toBeInTheDocument();
  });

  it('does not render activate/deactivate button for self', () => {
    const selfUser = { ...mockUser, id: 'current-user' };
    renderPanel(selfUser);
    expect(screen.queryByRole('button', { name: /desactivar|activar/i })).not.toBeInTheDocument();
  });

  it('renders no complexes message when empty', () => {
    renderPanel(mockUser, []);
    expect(screen.getByText('0 complejos')).toBeInTheDocument();
  });

  it('renders email verified badge when verified', () => {
    renderPanel();
    expect(screen.getByText(/verificado/i)).toBeInTheDocument();
  });

  it('confirms deactivation with the destructive style, matching the trigger button', async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.click(screen.getByRole('button', { name: /desactivar/i }));
    expect(await screen.findByRole('alertdialog')).toHaveAttribute('data-variant', 'destructive');
  });

  it('confirms reactivation without the destructive style', async () => {
    const user = userEvent.setup();
    renderPanel({ ...mockUser, is_active: false });
    await user.click(screen.getByRole('button', { name: /activar/i }));
    expect(await screen.findByRole('alertdialog')).toHaveAttribute('data-variant', 'default');
  });

  // Finding M12: this was a `<button onClick={() => navigate(...)}>`, so
  // there was no href to hover or open in a new tab. It is now a real link.
  it('links back to the users list', () => {
    renderPanel();
    const link = screen.getByRole('link', { name: t.admin.detail.backToUsers });
    expect(link).toHaveAttribute('href', '/admin/users');
  });
});
