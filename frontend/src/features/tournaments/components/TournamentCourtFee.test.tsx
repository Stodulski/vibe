import { describe, it, expect, vi, beforeEach } from 'vitest';
import { toast } from 'sonner';
import { makeConsumedHttpError } from '@/test/factories';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import { makeCategory, makeDrawnDetail, makeMatch } from '../testing/fixtures';
import { TournamentDetailContent } from '../pages/TournamentDetailPage';
import type { TournamentDetailResponse } from '@/shared/types/api.types';

vi.mock('../api/tournaments.api', () => ({
  tournamentsApi: {
    get: vi.fn(),
    cancel: vi.fn(),
    cancelCategory: vi.fn(),
    unschedule: vi.fn(),
    setCourtFee: vi.fn(),
    chargeCourtFee: vi.fn(),
    undoCourtFee: vi.fn(),
  },
}));
vi.mock('@/features/courts', () => ({ useCourts: () => ({ data: [] }) }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.tournaments;
const FEE = 1500000;

/** A drawn category with a court fee and two group matches between known pairs, one already charged on side 1. */
function detailWithFee(overrides: Partial<TournamentDetailResponse> = {}): TournamentDetailResponse {
  const detail = makeDrawnDetail();
  detail.categories = [makeCategory({ status: 'closed', draw_generated: true, confirmed_count: 4, court_fee: FEE })];
  detail.matches = [
    makeMatch({
      id: 'gm1',
      stage: 'group',
      groupId: 'g1',
      pair1Id: 'p1',
      pair2Id: 'p2',
      courtFee: { due: FEE, charged: { 1: { amount: 1000000, method: 'cash' } } },
    }),
    makeMatch({
      id: 'gm2',
      stage: 'group',
      groupId: 'g1',
      round: 2,
      pair1Id: 'p1',
      pair2Id: 'p3',
      courtFee: { due: FEE },
    }),
  ];
  return { ...detail, ...overrides };
}

async function renderOnMatches(detail: TournamentDetailResponse) {
  const { tournamentsApi } = await import('../api/tournaments.api');
  vi.mocked(tournamentsApi.get).mockResolvedValue(detail);
  renderWithProviders(<TournamentDetailContent complexId="c1" tournamentId="t1" />);
  await screen.findByRole('heading', { name: detail.tournament.name });
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: t.detail.sections.matches }));
  return { tournamentsApi, user };
}

describe('TournamentDetail: court fee', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('says the fee on the category and on every match, and what is collected and pending', async () => {
    await renderOnMatches(detailWithFee());

    expect(screen.getAllByText(t.courtFee.line(formatPrice(FEE)))).toHaveLength(2);
    // Collected is the charge at the amount it was made; pending the three sides still owing at the current fee.
    expect(screen.getByText(t.courtFee.summary(formatPrice(1000000), formatPrice(FEE * 3)))).toBeInTheDocument();
  });

  it('keeps a charge at the amount it was made when the fee has changed since', async () => {
    await renderOnMatches(detailWithFee());

    expect(screen.getByText(t.courtFee.paid(formatPrice(1000000)))).toBeInTheDocument();
    expect(screen.queryByText(t.courtFee.paid(formatPrice(FEE)))).not.toBeInTheDocument();
  });
});

describe('TournamentDetail: charging the court fee', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('charges a pair by the method chosen and shows the updated match', async () => {
    const { tournamentsApi, user } = await renderOnMatches(detailWithFee());
    const after = detailWithFee();
    after.matches = [
      ...after.matches.slice(0, 1),
      makeMatch({
        id: 'gm2',
        stage: 'group',
        groupId: 'g1',
        round: 2,
        pair1Id: 'p1',
        pair2Id: 'p3',
        courtFee: { due: FEE, charged: { 2: { amount: FEE, method: 'transfer' } } },
      }),
    ];
    vi.mocked(tournamentsApi.chargeCourtFee).mockResolvedValue(after);

    // Side 2 of the second match: Díaz / Vega.
    await user.click(screen.getByRole('button', { name: t.courtFee.chargeAria('Díaz / Vega') }));
    const dialog = await screen.findByRole('dialog', { name: t.courtFee.chargeTitle });
    expect(within(dialog).getByText(t.courtFee.chargeDescription('Díaz / Vega', formatPrice(FEE)))).toBeInTheDocument();
    await user.click(within(dialog).getByRole('combobox', { name: t.courtFee.method }));
    await user.click(await screen.findByRole('option', { name: ES_AR.bookings.paymentMethods.transfer }));
    await user.click(within(dialog).getByRole('button', { name: t.courtFee.chargeSubmit(formatPrice(FEE)) }));

    await waitFor(() => {
      expect(tournamentsApi.chargeCourtFee).toHaveBeenCalledWith('c1', 't1', 'gm2', 2, { method: 'transfer' });
    });
    expect(await screen.findAllByText(t.courtFee.paid(formatPrice(FEE)))).not.toHaveLength(0);
    expect(toast.success).toHaveBeenCalledWith(t.courtFee.chargeSuccess);
  });

  it('collects by cash unless told otherwise', async () => {
    const { tournamentsApi, user } = await renderOnMatches(detailWithFee());
    vi.mocked(tournamentsApi.chargeCourtFee).mockResolvedValue(detailWithFee());

    await user.click(screen.getByRole('button', { name: t.courtFee.chargeAria('Ruiz / Soto') }));
    await user.click(await screen.findByRole('button', { name: t.courtFee.chargeSubmit(formatPrice(FEE)) }));

    await waitFor(() => {
      expect(tournamentsApi.chargeCourtFee).toHaveBeenCalledWith('c1', 't1', 'gm1', 2, { method: 'cash' });
    });
  });
});

describe('TournamentDetail: refused court fee charges and undoing', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('tells in Spanish when no cash session is open, and leaves the dialog open', async () => {
    const { tournamentsApi, user } = await renderOnMatches(detailWithFee());
    vi.mocked(tournamentsApi.chargeCourtFee).mockRejectedValue(
      await makeConsumedHttpError(409, { title: 'x', detail: 'no cash session is open' }),
    );

    await user.click(screen.getByRole('button', { name: t.courtFee.chargeAria('Ruiz / Soto') }));
    await user.click(await screen.findByRole('button', { name: t.courtFee.chargeSubmit(formatPrice(FEE)) }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(t.courtFee.errors.chargeUnknown, {
        description: ES_AR.validation.server.cashClosed,
      });
    });
    expect(screen.getByRole('dialog', { name: t.courtFee.chargeTitle })).toBeInTheDocument();
  });

  it('undoes a charge after confirming', async () => {
    const { tournamentsApi, user } = await renderOnMatches(detailWithFee());
    vi.mocked(tournamentsApi.undoCourtFee).mockResolvedValue(detailWithFee());

    await user.click(screen.getByRole('button', { name: t.courtFee.undoAria('Pérez / Gómez') }));
    expect(tournamentsApi.undoCourtFee).not.toHaveBeenCalled();
    await user.click(await screen.findByRole('button', { name: t.courtFee.undoConfirm }));

    await waitFor(() => {
      expect(tournamentsApi.undoCourtFee).toHaveBeenCalledWith('c1', 't1', 'gm1', 1);
    });
    expect(toast.success).toHaveBeenCalledWith(t.courtFee.undoSuccess);
  });
});

describe('TournamentDetail: who owes the court fee', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('offers no charge for a walkover, a bye, a pair not known yet or a category without a fee', async () => {
    const detail = detailWithFee();
    detail.matches = [
      makeMatch({
        id: 'w',
        stage: 'group',
        groupId: 'g1',
        pair1Id: 'p1',
        pair2Id: 'p2',
        status: 'completed',
        walkoverLoser: 2,
        courtFee: { due: FEE },
      }),
      makeMatch({ id: 'b1', round: 1, position: 1, source1: { group_id: 'g1', rank: 1 }, courtFee: { due: FEE } }),
    ];
    await renderOnMatches(detail);
    expect(screen.queryByRole('button', { name: t.courtFee.charge })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Cobrar la cancha/ })).not.toBeInTheDocument();
  });

  it('shows no court fee at all in a category without one', async () => {
    const detail = detailWithFee();
    detail.categories = [makeCategory({ status: 'closed', draw_generated: true, court_fee: 0 })];
    detail.matches = [makeMatch({ id: 'gm1', stage: 'group', groupId: 'g1', pair1Id: 'p1', pair2Id: 'p2' })];
    await renderOnMatches(detail);

    expect(screen.queryByText(/^Cancha:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Cobrado /)).not.toBeInTheDocument();
    expect(screen.getByText(t.courtFee.none)).toBeInTheDocument();
  });
});

describe('TournamentDetail: editing the court fee', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('raises the fee from the category in pesos and sends centavos', async () => {
    const { tournamentsApi, user } = await renderOnMatches(detailWithFee());
    vi.mocked(tournamentsApi.setCourtFee).mockResolvedValue(detailWithFee());

    await user.click(screen.getByRole('button', { name: t.courtFee.edit }));
    const dialog = await screen.findByRole('dialog', { name: t.courtFee.editTitle });
    const field = within(dialog).getByLabelText(t.courtFee.editLabel);
    expect(field).toHaveValue('15.000');
    await user.clear(field);
    await user.type(field, '18000');
    await user.click(within(dialog).getByRole('button', { name: t.courtFee.editSubmit }));

    await waitFor(() => {
      expect(tournamentsApi.setCourtFee).toHaveBeenCalledWith('c1', 't1', 'cat1', { court_fee: 1800000 });
    });
    expect(toast.success).toHaveBeenCalledWith(t.courtFee.editSuccess);
  });

  it('turns the fee off with zero', async () => {
    const { tournamentsApi, user } = await renderOnMatches(detailWithFee());
    vi.mocked(tournamentsApi.setCourtFee).mockResolvedValue(detailWithFee());

    await user.click(screen.getByRole('button', { name: t.courtFee.edit }));
    const dialog = await screen.findByRole('dialog', { name: t.courtFee.editTitle });
    const field = within(dialog).getByLabelText(t.courtFee.editLabel);
    await user.clear(field);
    await user.type(field, '0');
    await user.click(within(dialog).getByRole('button', { name: t.courtFee.editSubmit }));

    await waitFor(() => {
      expect(tournamentsApi.setCourtFee).toHaveBeenCalledWith('c1', 't1', 'cat1', { court_fee: 0 });
    });
  });

  it('cannot be edited once the category is cancelled', async () => {
    const detail = detailWithFee();
    detail.categories = [makeCategory({ status: 'cancelled', court_fee: FEE })];
    await renderOnMatches(detail);

    expect(screen.queryByRole('button', { name: t.courtFee.edit })).not.toBeInTheDocument();
  });
});
