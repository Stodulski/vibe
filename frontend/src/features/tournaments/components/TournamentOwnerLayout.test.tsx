import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@testing-library/react';
import { renderWithProviders, screen, userEvent, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeCategory, makeDetail, makeDrawnDetail, makeMatch, makePair } from '../testing/fixtures';
import { TournamentDetailContent } from '../pages/TournamentDetailPage';
import { GroupsSection } from './GroupsSection';
import { MatchesSection } from './MatchesSection';
import { MatchCourtFee } from './MatchCourtFee';
import { PairsSection } from './PairsSection';
import type { TournamentDetailResponse } from '@/shared/types/api.types';

vi.mock('../api/tournaments.api', () => ({
  tournamentsApi: { get: vi.fn(), cancel: vi.fn(), cancelCategory: vi.fn(), unschedule: vi.fn() },
}));
vi.mock('@/features/courts', () => ({ useCourts: () => ({ data: [] }) }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.tournaments;

async function renderDetail(detail: TournamentDetailResponse) {
  const { tournamentsApi } = await import('../api/tournaments.api');
  vi.mocked(tournamentsApi.get).mockResolvedValue(detail);
  renderWithProviders(<TournamentDetailContent complexId="c1" tournamentId="t1" />);
  await screen.findByRole('heading', { name: detail.tournament.name });
  return userEvent.setup();
}

describe('owner tournament layout', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('gives every zone table the same fixed columns', () => {
    const detail = makeDrawnDetail();
    const pairs = new Map(detail.pairs.map((p) => [p.id, p]));
    render(<GroupsSection groups={detail.groups} pairs={pairs} qualifiers={2} />);
    for (const table of screen.getAllByRole('table')) {
      expect(table.className).toContain('table-fixed');
      expect(table.querySelectorAll('colgroup > col')).toHaveLength(7);
    }
  });

  it('shows the winner and walkover words proportionally, not in monospace', () => {
    const detail = makeDrawnDetail();
    const pairs = new Map(detail.pairs.map((p) => [p.id, p]));
    const match = makeMatch({
      id: 'm1',
      pair1Id: 'p1',
      pair2Id: 'p2',
      status: 'completed',
      winnerPairId: 'p1',
      walkoverLoser: 2,
    });
    render(
      <MatchesSection
        matches={[match]}
        pairs={pairs}
        groupNames={new Map()}
        editable={false}
        actions={{ onSchedule: vi.fn(), onUnschedule: vi.fn(), onResult: vi.fn() }}
        courtFee={{ fee: 0, actions: { onCharge: vi.fn(), onUndo: vi.fn() } }}
      />,
    );
    const words = screen.getByText(/^(Ganó|W\.O\.)/);
    expect(words.className).not.toContain('score-text');
    expect(words.className).toContain('tabular-nums');
  });

  it('shows both cancellations as visible destructive outline buttons, with no options menus', async () => {
    await renderDetail(makeDetail());
    for (const name of [t.cancel.action, t.cancel.categoryAction]) {
      expect(screen.getByRole('button', { name })).toHaveAttribute('data-variant', 'destructive-outline');
    }
    expect(t.cancel.action).not.toBe(t.cancel.categoryAction);
    expect(screen.queryByRole('button', { name: /^Opciones/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: ES_AR.common.rowActionsLabel })).not.toBeInTheDocument();
  });

  it('writes the tournament days with a capital letter on their own line', async () => {
    await renderDetail(makeDetail());
    const days = screen.getByText(
      /^(L|l)unes|^(S|s)ábado|^(D|d)omingo|^(V|v)iernes|^(J|j)ueves|^(M|m)i[eé]rcoles|^(M|m)artes/,
    );
    expect(days.className).toContain('block');
    expect(days.className).toContain('first-letter:uppercase');
  });
});

describe('owner tournament cancelled and facts layout', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('marks a cancelled category tab as muted', async () => {
    const detail = makeDetail({
      categories: [
        makeCategory({ id: 'a', name: 'Damas 5ta', status: 'cancelled' }),
        makeCategory({ id: 'b', name: '6ta' }),
      ],
    });
    await renderDetail(detail);
    const tab = screen.getByRole('button', { name: `Damas 5ta · ${t.categoryStatus.cancelled}` });
    expect(tab).toHaveAttribute('data-cancelled', 'true');
    expect(tab.className).toContain('text-text-tertiary');
  });

  it('keeps the cancelled note after the facts so the tabs do not move', async () => {
    const detail = makeDetail({ categories: [makeCategory({ status: 'cancelled' })] });
    await renderDetail(detail);
    const note = screen.getByText(t.detail.categoryCancelledNote);
    const capacity = screen.getByText(t.detail.capacity);
    expect(capacity.compareDocumentPosition(note) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('shows all money in the facts proportionally with tabular numbers', async () => {
    await renderDetail(makeDetail({ categories: [makeCategory({ court_fee: 300000 })] }));
    const price = screen.getByText('$20.000');
    expect(price.className).not.toContain('score-text');
    expect(price.className).toContain('tabular-nums');
    const fee = screen.getByText('$3.000');
    expect(fee.className).not.toContain('score-text');
  });

  it('labels the fee action "Agregar" when the category has no court fee', async () => {
    await renderDetail(makeDetail());
    expect(screen.getByRole('button', { name: t.courtFee.add })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.courtFee.edit })).not.toBeInTheDocument();
  });

  it('describes the draw without zones for an elimination category', async () => {
    await renderDetail(makeDetail({ categories: [makeCategory({ format: 'elimination' })] }));
    expect(screen.getByText(t.draw.generateHintElimination)).toBeInTheDocument();
    expect(t.draw.generateHintElimination).not.toMatch(/zona/i);
  });

  it('says "Semifinales" for a round of several matches and "Semifinal" for one', () => {
    expect(t.bracket.semifinals).toBe('Semifinales');
    expect(t.bracket.semifinal).toBe('Semifinal');
  });
});

describe('MatchCourtFee actions', () => {
  it('styles charge and undo the same, in a two-column row', () => {
    const pairs = new Map([
      ['p1', makePair({ id: 'p1' })],
      ['p2', makePair({ id: 'p2', players: ['Carla Ruiz', 'Dana Soto'] })],
    ]);
    const match = makeMatch({
      id: 'm1',
      pair1Id: 'p1',
      pair2Id: 'p2',
      courtFee: { due: 200000, charged: { 2: { amount: 200000, method: 'cash' } } },
    });
    render(
      <MatchCourtFee
        match={match}
        fee={200000}
        pairs={pairs}
        groupNames={new Map()}
        actions={{ onCharge: vi.fn(), onUndo: vi.fn() }}
        editable
      />,
    );
    const charge = screen.getByRole('button', { name: /^Cobrar la cancha/ });
    const undo = screen.getByRole('button', { name: /^Deshacer el cobro/ });
    expect(charge.className).toBe(undo.className);
    expect(charge.className).not.toContain('underline');
    const row = charge.closest('li');
    expect(row?.className).toContain('grid');
    expect(within(row as HTMLElement).getByText(/Pérez/)).toBeInTheDocument();
  });
});

describe('PairsSection phone links', () => {
  it('exempts the phones from the touch height and aligns the payment state to the name baseline', () => {
    render(<PairsSection pairs={[makePair({ id: 'p1' })]} groupNames={new Map()} canSwap={false} onSwap={vi.fn()} />);
    const link = screen.getAllByRole('link')[0];
    expect(link?.className).toContain('inline-link');
    const state = screen.getByText(/^Pagó/);
    expect(state.closest('li')?.className).toContain('items-baseline');
  });
});
