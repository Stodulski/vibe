import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ProfileChecklist } from './ProfileChecklist';
import { renderWithProviders } from '@/test/test-utils';
import { useCourts } from '@/features/courts';
import { makeActivePricedCourt, makeCompleteComplex } from '@/test/factories';
import type { Complex, CourtWithPrices } from '@/shared/types/api.types';

// Mocked at the hook file: the feature barrels re-export these, so the component
// under test picks up the mock through the public import path.
vi.mock('@/features/courts/hooks/useCourts', () => ({ useCourts: vi.fn() }));

const EXPANDED_KEY = 'vibe.profileChecklist.expanded';
const WHY_PHOTOS = 'Las fotos hacen que más jugadores elijan tu complejo.';

function mockQueries({ courts }: { courts?: CourtWithPrices[] | undefined }) {
  vi.mocked(useCourts).mockReturnValue({ data: courts } as unknown as ReturnType<typeof useCourts>);
}

function renderChecklist(complex: Complex) {
  return renderWithProviders(<ProfileChecklist complex={complex} />);
}

/** Cover, logo, description and priced court open; map, services, phone and MercadoPago done. */
function fourOpen() {
  mockQueries({ courts: [] });
  return makeCompleteComplex({ cover_url: null, logo_url: null, description: null });
}

const WHY_MERCADOPAGO = 'Con MercadoPago conectado, los jugadores pueden pagar la seña al reservar.';

beforeEach(() => {
  vi.clearAllMocks();
  window.localStorage.clear();
});

describe('ProfileChecklist: collapsed', () => {
  it('renders nothing while the courts are still loading', () => {
    mockQueries({ courts: undefined });

    const { container } = renderChecklist(makeCompleteComplex());

    expect(container).toBeEmptyDOMElement();
  });

  it('collapsed, shows the progress and only the next pending item', () => {
    renderChecklist(fourOpen());

    expect(screen.getByRole('heading', { name: 'Completá tu perfil' })).toBeInTheDocument();
    expect(screen.getByText('4 de 8 listos')).toBeInTheDocument();
    expect(screen.getByText(WHY_PHOTOS)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Subí la foto de portada' })).toHaveAttribute('href', '/settings');
    expect(screen.queryByRole('link', { name: 'Escribí tu descripción' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Subí tu logo' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Cargá los precios' })).not.toBeInTheDocument();
  });

  it('the disclosure reports the pending count and starts collapsed', () => {
    renderChecklist(fourOpen());

    const toggle = screen.getByRole('button', { name: 'Ver los 4 pendientes' });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
  });

  it('skips completed items when choosing the next one', () => {
    mockQueries({ courts: [makeActivePricedCourt()] });

    renderChecklist(makeCompleteComplex({ cover_url: 'https://cdn.example.com/c.jpg', description: null }));

    expect(screen.getByRole('link', { name: 'Escribí tu descripción' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Subí la foto de portada' })).not.toBeInTheDocument();
  });

  it('shows MercadoPago as the next step once everything else is done but payments are off', () => {
    mockQueries({ courts: [makeActivePricedCourt()] });

    renderChecklist(makeCompleteComplex({ payments_enabled: false }));

    expect(screen.getByText('Conectá MercadoPago para cobrar señas online')).toBeInTheDocument();
    expect(screen.getByText(WHY_MERCADOPAGO)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Conectá tu cuenta de MercadoPago' })).toHaveAttribute(
      'href',
      '/settings?tab=billing',
    );
  });
});

describe('ProfileChecklist: expanded', () => {
  it('lists MercadoPago last, after the priced court, with a link to the billing tab', async () => {
    const user = userEvent.setup();
    mockQueries({ courts: [] });
    renderChecklist(makeCompleteComplex({ cover_url: null, description: null, payments_enabled: false }));

    await user.click(screen.getByRole('button', { name: 'Ver los 4 pendientes' }));

    const rows = within(screen.getByRole('list')).getAllByRole('listitem');
    expect(rows).toHaveLength(4);
    expect(rows[3]).toHaveTextContent('Conectá MercadoPago para cobrar señas online');
    expect(screen.getByRole('link', { name: 'Conectá tu cuenta de MercadoPago' })).toHaveAttribute(
      'href',
      '/settings?tab=billing',
    );
  });

  it('expanding lists every pending item once, each row a link to its destination', async () => {
    const user = userEvent.setup();
    renderChecklist(fourOpen());

    await user.click(screen.getByRole('button', { name: 'Ver los 4 pendientes' }));

    expect(screen.getByRole('button', { name: 'Ocultar pendientes' })).toHaveAttribute('aria-expanded', 'true');
    const list = screen.getByRole('list');
    const rows = within(list).getAllByRole('listitem');
    expect(rows).toHaveLength(4);
    expect(within(list).getByText('Foto de portada')).toBeInTheDocument();
    expect(within(list).getByText('Descripción')).toBeInTheDocument();
    expect(within(list).getByText('Logo')).toBeInTheDocument();
    expect(within(list).getByText('Cancha con precio')).toBeInTheDocument();
    expect(within(list).queryByText('Revisá tus horarios')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Subí la foto de portada' })).toHaveAttribute('href', '/settings');
    expect(screen.getByRole('link', { name: 'Escribí tu descripción' })).toHaveAttribute('href', '/settings');
    expect(screen.getByRole('link', { name: 'Subí tu logo' })).toHaveAttribute('href', '/settings');
    expect(screen.getByRole('link', { name: 'Cargá los precios' })).toHaveAttribute('href', '/courts');
    // The whole row is the link, label included, not only the chevron.
    expect(screen.getByRole('link', { name: 'Subí la foto de portada' })).toContainElement(
      within(list).getByText('Foto de portada'),
    );
  });

  it('expanded rows carry no per-item reason text', async () => {
    const user = userEvent.setup();
    renderChecklist(fourOpen());

    await user.click(screen.getByRole('button', { name: 'Ver los 4 pendientes' }));

    expect(screen.queryByText(WHY_PHOTOS)).not.toBeInTheDocument();
  });

  it('collapsing again hides the list and brings back the next item', async () => {
    const user = userEvent.setup();
    renderChecklist(fourOpen());
    await user.click(screen.getByRole('button', { name: 'Ver los 4 pendientes' }));

    await user.click(screen.getByRole('button', { name: 'Ocultar pendientes' }));

    expect(screen.queryByRole('list')).not.toBeInTheDocument();
    expect(screen.getByText(WHY_PHOTOS)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Ver los 4 pendientes' })).toHaveAttribute('aria-expanded', 'false');
  });

  it('shows no disclosure when only one item is pending, because the next item already is that item', () => {
    mockQueries({ courts: [makeActivePricedCourt()] });

    renderChecklist(makeCompleteComplex({ description: null }));

    expect(screen.getByRole('link', { name: 'Escribí tu descripción' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /pendientes/ })).not.toBeInTheDocument();
  });
});

describe('ProfileChecklist: state', () => {
  it('remembers an expanded choice per browser', async () => {
    const user = userEvent.setup();
    window.localStorage.setItem(EXPANDED_KEY, '1');
    renderChecklist(fourOpen());

    expect(screen.getByRole('button', { name: 'Ocultar pendientes' })).toHaveAttribute('aria-expanded', 'true');

    await user.click(screen.getByRole('button', { name: 'Ocultar pendientes' }));

    expect(window.localStorage.getItem(EXPANDED_KEY)).toBe('0');
  });

  it('exposes progress as a progressbar with the completed count', () => {
    renderChecklist(fourOpen());

    const bar = screen.getByRole('progressbar');
    expect(bar).toHaveAttribute('aria-valuenow', '4');
    expect(bar).toHaveAttribute('aria-valuemax', '8');
  });

  it('renders nothing once every item is done', () => {
    mockQueries({ courts: [makeActivePricedCourt()] });

    const { container } = renderChecklist(makeCompleteComplex());

    expect(container).toBeEmptyDOMElement();
  });
});
