import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/test-utils';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeCategory, makeDetail, makeMatch } from '../testing/fixtures';
import { ScheduleMatchDialog } from './ScheduleMatchDialog';
import type { CourtWithPrices, TournamentMatch } from '@/shared/types/api.types';

vi.mock('../api/tournaments.api', () => ({ tournamentsApi: { schedule: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));
vi.mock('@/features/complex', () => ({
  useSelectedComplex: () => ({ complex: { id: 'c1', slug: 'club' } }),
  useSchedules: () => ({ data: undefined }),
}));

const t = ES_AR.tournaments.schedule;
const { tournament } = makeDetail();

function court(id: string, name: string): CourtWithPrices {
  return {
    id,
    complex_id: 'c1',
    name,
    sport: 'padel',
    court_type: 'outdoor',
    is_active: true,
    has_lighting: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices: [],
  };
}

const COURTS = [court('ct1', 'Cancha 1'), court('ct2', 'Cancha 2')];
const SCHEDULED = makeMatch({
  id: 'm1',
  pair1Id: 'p1',
  pair2Id: 'p2',
  schedule: {
    court_id: 'ct1',
    court_name: 'Cancha 1',
    date: '2099-01-03',
    start_time: '18:00',
    end_time: '19:00',
    duration_minutes: 60,
  },
});

async function renderDialog(match: TournamentMatch, duration = 90) {
  const { tournamentsApi } = await import('../api/tournaments.api');
  vi.mocked(tournamentsApi.schedule).mockResolvedValue({ match });
  const onClose = vi.fn();
  renderWithProviders(
    <ScheduleMatchDialog
      open
      onClose={onClose}
      complexId="c1"
      tournament={tournament}
      category={makeCategory({ match_duration_minutes: duration })}
      match={match}
      courts={COURTS}
    />,
  );
  return { tournamentsApi, onClose, user: userEvent.setup() };
}

describe('ScheduleMatchDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('prefills the duration with the category match duration', async () => {
    await renderDialog(makeMatch({ id: 'm1', pair1Id: 'p1', pair2Id: 'p2' }), 75);

    expect(screen.getByLabelText(t.duration)).toHaveValue(75);
    expect(screen.getByText(t.durationHint(75))).toBeInTheDocument();
  });

  it('asks for the court, day and time when sent empty, and sends nothing', async () => {
    const { tournamentsApi, user } = await renderDialog(makeMatch({ id: 'm1', pair1Id: 'p1', pair2Id: 'p2' }));

    await user.click(screen.getByRole('button', { name: t.submit }));

    expect(await screen.findByText(t.courtRequired)).toBeInTheDocument();
    expect(screen.getByText(ES_AR.validation.dateRequired)).toBeInTheDocument();
    expect(screen.getByText(ES_AR.validation.timeRequired)).toBeInTheDocument();
    expect(tournamentsApi.schedule).not.toHaveBeenCalled();
  });

  it('offers only the days of the tournament in the date picker', async () => {
    const { user } = await renderDialog(makeMatch({ id: 'm1', pair1Id: 'p1', pair2Id: 'p2' }));

    await user.click(screen.getByRole('button', { name: new RegExp(t.date) }));

    const grid = await screen.findByRole('grid');
    const day = (n: string) => within(grid).getByRole('button', { name: new RegExp(`^\\w+, ${n} de enero`) });
    // The tournament runs 2099-01-02 to 2099-01-04.
    expect(day('1')).toBeDisabled();
    expect(day('2')).toBeEnabled();
    expect(day('4')).toBeEnabled();
    expect(day('5')).toBeDisabled();
  });

  it('opens a scheduled match with its court, day, time and duration, and sends the change', async () => {
    const { tournamentsApi, onClose, user } = await renderDialog(SCHEDULED);
    expect(screen.getByLabelText(t.duration)).toHaveValue(60);

    await user.clear(screen.getByLabelText(t.duration));
    await user.type(screen.getByLabelText(t.duration), '120');
    await user.click(screen.getByRole('button', { name: t.submit }));

    await waitFor(() => {
      expect(tournamentsApi.schedule).toHaveBeenCalledWith('c1', 't1', 'm1', {
        court_id: 'ct1',
        date: '2099-01-03',
        start_time: '18:00',
        duration_minutes: 120,
      });
    });
    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('names what holds the hours, in Spanish, when the court is taken', async () => {
    const { tournamentsApi, onClose, user } = await renderDialog(SCHEDULED);
    vi.mocked(tournamentsApi.schedule).mockRejectedValue(
      await makeConsumedHttpError(409, {
        title: 'Conflict',
        detail: 'a booking already holds those hours on Cancha 1',
      }),
    );

    await user.click(screen.getByRole('button', { name: t.submit }));

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent(t.conflict.title);
    expect(alert).toHaveTextContent(t.conflict.booking('Cancha 1'));
    expect(onClose).not.toHaveBeenCalled();
  });
});
