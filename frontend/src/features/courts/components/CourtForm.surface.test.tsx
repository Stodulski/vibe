import { describe, it, expect, vi, beforeEach } from 'vitest';
import userEvent from '@testing-library/user-event';
import { renderWithProviders, screen, waitFor, within } from '@/test/test-utils';
import { makeCourt } from '@/test/factories';
import { CourtForm } from './CourtForm';

const mockCreateMutate = vi.fn();
const mockUpdateMutate = vi.fn();

vi.mock('../hooks/useCreateCourt', () => ({
  useCreateCourt: () => ({
    mutate: mockCreateMutate,
    isPending: false,
  }),
}));

vi.mock('../hooks/useUpdateCourt', () => ({
  useUpdateCourt: () => ({
    mutate: mockUpdateMutate,
    isPending: false,
  }),
}));

async function openSurfaceOptions(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('combobox', { name: 'Superficie' }));
  return within(await screen.findByRole('listbox'));
}

async function pickSport(user: ReturnType<typeof userEvent.setup>, label: string) {
  await user.click(screen.getByRole('combobox', { name: 'Deporte' }));
  await user.click(await screen.findByRole('option', { name: label }));
}

const defaultProps = { open: true, onClose: vi.fn(), complexId: 'c1' };

describe('CourtForm surface options', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('lists only the surfaces of the selected sport, plus an empty option', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CourtForm {...defaultProps} />);

    let list = await openSurfaceOptions(user);
    expect(list.getAllByRole('option').map((o) => o.textContent)).toEqual([
      'Sin especificar',
      'Césped sintético',
      'Cemento',
    ]);
    await user.keyboard('{Escape}');

    await pickSport(user, 'Hockey');
    list = await openSurfaceOptions(user);
    expect(list.getAllByRole('option').map((o) => o.textContent)).toEqual(['Sin especificar', 'Césped sintético']);
  });

  it('clears a surface the new sport does not offer, and keeps one it does', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CourtForm {...defaultProps} court={{ ...makeCourt({ surface: 'concrete' }), prices: [] }} />);
    expect(screen.getByRole('combobox', { name: 'Superficie' })).toHaveTextContent('Cemento');

    await pickSport(user, 'Tenis');
    expect(screen.getByRole('combobox', { name: 'Superficie' })).toHaveTextContent('Cemento');

    await pickSport(user, 'Hockey');
    expect(screen.getByRole('combobox', { name: 'Superficie' })).toHaveTextContent('Sin especificar');
  });
});

describe('CourtForm surface and lighting submit', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('sends lighting and surface when creating, and no surface key when none is chosen', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CourtForm {...defaultProps} />);
    await user.type(screen.getByLabelText(/nombre/i), 'Cancha 9');
    await user.click(screen.getByRole('switch', { name: 'Tiene iluminación' }));
    await user.click(screen.getByRole('button', { name: /crear/i }));

    await waitFor(() => {
      expect(mockCreateMutate).toHaveBeenCalledTimes(1);
    });
    const sent = mockCreateMutate.mock.calls[0]?.[0] as Record<string, unknown>;
    expect(sent).toMatchObject({ name: 'Cancha 9', sport: 'padel', has_lighting: true });
    expect('surface' in sent).toBe(false);
  });

  it('sends the chosen surface when creating', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CourtForm {...defaultProps} />);
    await user.type(screen.getByLabelText(/nombre/i), 'Cancha 9');
    await user.click(
      await (async () => {
        await user.click(screen.getByRole('combobox', { name: 'Superficie' }));
        return screen.findByRole('option', { name: 'Cemento' });
      })(),
    );
    await user.click(screen.getByRole('button', { name: /crear/i }));

    await waitFor(() => {
      expect(mockCreateMutate).toHaveBeenCalledTimes(1);
    });
    expect(mockCreateMutate.mock.calls[0]?.[0]).toMatchObject({ surface: 'concrete', has_lighting: false });
  });

  it('sends an empty surface on edit so the server clears it', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CourtForm {...defaultProps} court={{ ...makeCourt({ surface: 'concrete' }), prices: [] }} />);
    const list = await openSurfaceOptions(user);
    await user.click(list.getByRole('option', { name: 'Sin especificar' }));
    await user.click(screen.getByRole('button', { name: /guardar/i }));

    await waitFor(() => {
      expect(mockUpdateMutate).toHaveBeenCalledTimes(1);
    });
    expect(mockUpdateMutate.mock.calls[0]?.[0]).toMatchObject({
      courtId: 'ct1',
      data: { surface: '', has_lighting: false },
    });
  });
});
