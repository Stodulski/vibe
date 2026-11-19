import { makeCourt } from '@/test/factories';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, within } from '@/test/test-utils';
import { CourtCard } from './CourtCard';
import { CourtTable } from './CourtTable';
import type { CourtWithPrices } from '@/shared/types/api.types';
import type { ReactElement } from 'react';

const mutate = vi.fn();

vi.mock('../hooks/useUpdateCourt', () => ({
  useUpdateCourt: () => ({ mutate, isPending: false }),
}));

vi.mock('../hooks/useDeleteCourt', () => ({
  useDeleteCourt: () => ({ mutate: vi.fn(), isPending: false }),
}));

const active: CourtWithPrices = { ...makeCourt({ id: 'ct1', name: 'Cancha 1', is_active: true }), prices: [] };
const inactive: CourtWithPrices = { ...makeCourt({ id: 'ct1', name: 'Cancha 1', is_active: false }), prices: [] };

const callbacks = { complexId: 'c1', onEdit: vi.fn(), onPrices: vi.fn() };

const renderers: [string, (court: CourtWithPrices) => ReactElement][] = [
  ['CourtCard', (court) => <CourtCard court={court} {...callbacks} />],
  ['CourtTable', (court) => <CourtTable courts={[court]} {...callbacks} />],
];

describe.each(renderers)('deactivating a court from %s', (_name, view) => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('asks first, naming the court, and changes nothing yet', async () => {
    const user = userEvent.setup();
    renderWithProviders(view(active));

    await user.click(screen.getByRole('switch'));

    const dialog = await screen.findByRole('alertdialog');
    expect(within(dialog).getByText(/^¿Desactivar Cancha\s1\?$/)).toBeInTheDocument();
    expect(mutate).not.toHaveBeenCalled();
  });

  it('deactivates only once confirmed', async () => {
    const user = userEvent.setup();
    renderWithProviders(view(active));

    await user.click(screen.getByRole('switch'));
    await user.click(await screen.findByRole('button', { name: 'Desactivar' }));

    expect(mutate).toHaveBeenCalledTimes(1);
    expect(mutate).toHaveBeenCalledWith({ courtId: 'ct1', data: { is_active: false } }, expect.anything());
  });

  it('keeps the court active when the owner dismisses with Cancelar', async () => {
    const user = userEvent.setup();
    renderWithProviders(view(active));

    await user.click(screen.getByRole('switch'));
    await user.click(await screen.findByRole('button', { name: 'Cancelar' }));

    expect(mutate).not.toHaveBeenCalled();
  });

  it('reactivates without asking', async () => {
    const user = userEvent.setup();
    renderWithProviders(view(inactive));

    await user.click(screen.getByRole('switch'));

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
    expect(mutate).toHaveBeenCalledWith({ courtId: 'ct1', data: { is_active: true } });
  });
});
