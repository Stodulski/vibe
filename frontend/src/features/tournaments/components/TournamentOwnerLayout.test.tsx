import { tournamentTitle } from '@/shared/lib/traitLabels';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@testing-library/react';
import { renderWithProviders, screen, userEvent, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import { makeCategory, makeDetail, makeDrawnDetail, makeMatch, makePair } from '../testing/fixtures';
import { pickSection } from '../testing/sectionSelect';
import { TournamentDetailContent } from '../pages/TournamentDetailPage';
import { GroupsSection } from './GroupsSection';
import { MatchesSection } from './MatchesSection';
import { MatchCourtFee } from './MatchCourtFee';
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
  await screen.findByRole('heading', { name: tournamentTitle(detail.tournament.sport) });
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
      expect(table.querySelectorAll('thead th')).toHaveLength(8);
      for (const row of table.querySelectorAll('tbody tr')) {
        expect(row.querySelectorAll('td')).toHaveLength(8);
      }
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
});

describe('owner tournament match rows', () => {
  it('keeps a match schedule on one truncated line with the day in figures', () => {
    const detail = makeDrawnDetail();
    const pairs = new Map(detail.pairs.map((p) => [p.id, p]));
    const schedule = {
      court_id: 'ct1',
      court_name: 'Pádel 3',
      date: '2026-11-22',
      start_time: '14:00',
      end_time: '15:30',
      duration_minutes: 90,
    };
    const match = makeMatch({ id: 'm1', pair1Id: 'p1', pair2Id: 'p2', schedule });
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
    expect(screen.getByText(/Dom 22\/11/)).toHaveClass('truncate');
    expect(screen.getByText(t.matches.versus)).toHaveClass('shrink-0');
    expect(screen.queryByRole('button', { name: t.matches.actionsLabel })).not.toBeInTheDocument();
  });
});

describe('owner tournament menus and tabs', () => {
  it('puts both cancellations in overflow menus, as destructive items, not as buttons', async () => {
    const user = await renderDetail(makeDetail());
    expect(screen.queryByRole('button', { name: t.cancel.action })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.cancel.categoryAction })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: ES_AR.common.rowActionsLabel }));
    expect(await screen.findByRole('menuitem', { name: t.cancel.action })).toHaveAttribute(
      'data-variant',
      'destructive',
    );
    await user.keyboard('{Escape}');

    await user.click(screen.getByRole('button', { name: t.detail.categoryActionsLabel }));
    expect(await screen.findByRole('menuitem', { name: t.cancel.categoryAction })).toHaveAttribute(
      'data-variant',
      'destructive',
    );
  });

  it('opens a category on its pairs before the draw', async () => {
    await renderDetail(makeDetail());
    expect(screen.getByRole('combobox', { name: t.detail.sectionLabel })).toHaveTextContent(t.detail.sections.pairs);
  });

  it('opens a drawn category on its matches', async () => {
    await renderDetail(makeDrawnDetail());
    expect(screen.getByRole('combobox', { name: t.detail.sectionLabel })).toHaveTextContent(t.detail.sections.matches);
  });

  it('switches the section from the select, and opens each category on its own section', async () => {
    const categories = [
      makeCategory({ id: 'a', modality: 'men', level: 6 }),
      makeCategory({ id: 'b', modality: 'women', level: 5, draw_generated: true }),
    ];
    const user = await renderDetail(makeDetail({ categories }));
    await pickSection(user, t.detail.sections.bracket);
    expect(screen.getByRole('combobox', { name: t.detail.sectionLabel })).toHaveTextContent(t.detail.sections.bracket);
    expect(screen.getByText(t.bracket.none)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Damas 5ta' }));
    expect(screen.getByRole('combobox', { name: t.detail.sectionLabel })).toHaveTextContent(t.detail.sections.matches);

    await user.click(screen.getByRole('button', { name: 'Caballeros 6ta' }));
    expect(screen.getByRole('combobox', { name: t.detail.sectionLabel })).toHaveTextContent(t.detail.sections.pairs);
  });

  it('sets the category select and the section select side by side from four categories', async () => {
    const categories = [1, 2, 3, 4].map((level) => makeCategory({ id: `c${String(level)}`, level }));
    await renderDetail(makeDetail({ categories }));
    const row = screen.getByRole('combobox', { name: t.detail.categoriesLabel }).closest('.grid');
    expect(row).toHaveClass('grid-cols-2');
    expect(row).toContainElement(screen.getByRole('combobox', { name: t.detail.sectionLabel }));
  });

  it('writes the tournament days with a capital letter on their own line', async () => {
    await renderDetail(makeDetail());
    const days = screen.getByText(
      /^(L|l)unes|^(S|s)ábado|^(D|d)omingo|^(V|v)iernes|^(J|j)ueves|^(M|m)i[eé]rcoles|^(M|m)artes/,
    );
    const line = days.closest('p');
    expect(line).toHaveClass('block', 'truncate', 'first-letter:uppercase');
  });
});

describe('owner tournament cancelled and facts layout', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('marks a cancelled category tab as muted', async () => {
    const detail = makeDetail({
      categories: [
        makeCategory({ id: 'a', modality: 'women', level: 5, status: 'cancelled' }),
        makeCategory({ id: 'b', modality: 'men', level: 6 }),
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
    // The summary carries its phone and desktop wording in two spans of one line; the first is enough for the order.
    const [summary] = screen.getAllByText(/6\/16 parejas/);
    if (!summary) throw new Error('the summary line is missing');
    expect(summary.compareDocumentPosition(note) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('gives the price in the summary line, with no disclosure to open', async () => {
    await renderDetail(makeDetail({ categories: [makeCategory({ court_fee: 300000 })] }));
    expect(screen.getByText(/\$20\.000 por pareja/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /reglas/ })).not.toBeInTheDocument();
  });

  it('labels the fee action "Agregar" when the category has no court fee', async () => {
    const user = await renderDetail(makeDetail());
    await user.click(screen.getByRole('button', { name: t.detail.categoryActionsLabel }));
    expect(await screen.findByRole('menuitem', { name: t.courtFee.add })).toBeInTheDocument();
    expect(screen.queryByRole('menuitem', { name: t.courtFee.edit })).not.toBeInTheDocument();
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

describe('MatchCourtFee', () => {
  it('shows what each pair paid or still owes as a status line, with no button in it', () => {
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
    render(<MatchCourtFee match={match} fee={200000} pairs={pairs} groupNames={new Map()} />);

    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    const row = screen.getByText(t.matches.pending).closest('li');
    expect(within(row as HTMLElement).getByText(/Pérez/)).toBeInTheDocument();
    expect(screen.getByText(t.courtFee.paid(formatPrice(200000)))).toBeInTheDocument();
  });
});
