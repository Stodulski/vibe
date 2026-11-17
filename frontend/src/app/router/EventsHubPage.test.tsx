import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { todayInArgentina } from '@/shared/lib/today';
import { pastEventsRange, upcomingEventsRange } from '@/features/events';
import { pastTournamentsRange } from '@/features/tournaments';
import { eventsApi } from '@/features/events/api/events.api';
import { tournamentsApi } from '@/features/tournaments/api/tournaments.api';
import { makeCategory, makeDetail } from '@/features/tournaments/testing/fixtures';
import type { OpenPlayEvent, TournamentOwnerSummary } from '@/shared/types/api.types';
import EventsHubPage from './EventsHubPage';

vi.mock('@/features/events/api/events.api', () => ({
  eventsApi: { list: vi.fn(), get: vi.fn(), cancel: vi.fn(), create: vi.fn() },
}));
vi.mock('@/features/tournaments/api/tournaments.api', () => ({ tournamentsApi: { list: vi.fn(), create: vi.fn() } }));
vi.mock('@/features/complex', () => ({
  useSelectedComplex: () => ({ selectedComplexId: 'c1', complex: { id: 'c1', slug: 'club' } }),
  useSchedules: () => ({ data: [] }),
}));
vi.mock('@/features/courts', () => ({ useCourts: () => ({ data: [] }) }));

const t = ES_AR.events;
const tt = ES_AR.tournaments;

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(eventsApi.list).mockResolvedValue({ events: [] });
  vi.mocked(tournamentsApi.list).mockResolvedValue({ tournaments: [] });
});

function makeEvent(overrides: Partial<OpenPlayEvent>): OpenPlayEvent {
  return {
    id: 'ev1',
    complex_id: 'c1',
    name: 'Americano de viernes',
    sport: 'padel',
    date: '2099-01-02',
    start_time: '19:00',
    end_time: '21:00',
    duration_minutes: 120,
    capacity: 20,
    price: 500000,
    status: 'active',
    courts: [{ id: 'ct1', name: 'Cancha 1' }],
    confirmed_count: 5,
    held_count: 0,
    spots_left: 15,
    created_at: '2026-11-14T12:00:00Z',
    ...overrides,
  };
}

function summary(id: string, name: string, startDate = '2099-01-02', status: 'active' | 'cancelled' = 'active') {
  const { tournament } = makeDetail();
  return {
    ...tournament,
    id,
    name,
    status,
    start_date: startDate,
    end_date: startDate,
    unscheduled_count: 0,
    categories: [
      makeCategory({ confirmed_count: 6 }),
      makeCategory({ id: 'cat2', name: 'Caballeros 7ma', level: 7, confirmed_count: 4 }),
    ],
  } as TournamentOwnerSummary;
}

describe('EventsHubPage: one list for both kinds', () => {
  it('lists the upcoming events of the complex: name, courts, spots taken and price', async () => {
    vi.mocked(eventsApi.list).mockResolvedValue({ events: [makeEvent({})] });
    renderWithProviders(<EventsHubPage />);

    const card = await screen.findByRole('button', { name: /Cancha abierta de pádel/ });
    expect(card).toHaveTextContent('Cancha 1');
    expect(card).toHaveTextContent(t.spotsTaken(5, 20));
    const { from, to } = upcomingEventsRange(todayInArgentina());
    expect(eventsApi.list).toHaveBeenCalledWith('c1', from, to, expect.anything());
  });

  it('mixes events and tournaments in one list, soonest first, each with its kind label', async () => {
    vi.mocked(eventsApi.list).mockResolvedValue({ events: [makeEvent({ date: '2099-01-03' })] });
    vi.mocked(tournamentsApi.list).mockResolvedValue({ tournaments: [summary('t1', 'Copa Vibe', '2099-01-02')] });
    renderWithProviders(<EventsHubPage />);

    const list = await screen.findByRole('list', { name: t.listLabel });
    const [first, second] = within(list).getAllByRole('listitem');
    if (!first || !second) throw new Error('expected two list items');
    expect(first).toHaveTextContent('Torneo de pádel');
    expect(within(first).getByRole('link')).toHaveAttribute('href', '/events/tournaments/t1');
    expect(second).toHaveTextContent('Cancha abierta de pádel');
  });

  it('has no Canchas abiertas or Torneos tabs', async () => {
    renderWithProviders(<EventsHubPage />);

    await screen.findByText(t.noUpcoming);
    expect(screen.queryByRole('navigation', { name: ES_AR.navigation.events })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: tt.title })).not.toBeInTheDocument();
  });
});

describe('EventsHubPage: badges and the empty state', () => {
  it('marks a cancelled event and a full one', async () => {
    vi.mocked(eventsApi.list).mockResolvedValue({
      events: [
        makeEvent({ id: 'a', modality: 'men', status: 'cancelled' }),
        makeEvent({ id: 'b', modality: 'women', spots_left: 0, confirmed_count: 20 }),
      ],
    });
    renderWithProviders(<EventsHubPage />);

    expect(await screen.findByRole('button', { name: /Caballeros/ })).toHaveTextContent(t.cancelledBadge);
    expect(screen.getByRole('button', { name: /Damas/ })).toHaveTextContent(t.fullBadge);
  });

  it('marks a cancelled tournament', async () => {
    vi.mocked(tournamentsApi.list).mockResolvedValue({
      tournaments: [summary('t1', 'Copa Vibe', '2099-01-02', 'cancelled')],
    });
    renderWithProviders(<EventsHubPage />);

    expect(await screen.findByText(tt.cancelledBadge)).toBeInTheDocument();
  });

  it('shows one empty state with the Crear evento action when both lists are empty', async () => {
    renderWithProviders(<EventsHubPage />);
    const user = userEvent.setup();

    const empty = await screen.findByRole('region', { name: t.noUpcoming });
    expect(screen.getAllByText(t.noUpcoming)).toHaveLength(1);
    await user.click(within(empty).getByRole('button', { name: t.create }));

    expect(await screen.findByRole('dialog', { name: ES_AR.events.typePicker.title })).toBeInTheDocument();
  });
});

describe('EventsHubPage: Pasados', () => {
  it('loads the past window of both kinds only on the Pasados tab, most recent first', async () => {
    vi.mocked(eventsApi.list).mockImplementation((_id, from) =>
      Promise.resolve({
        events:
          from === upcomingEventsRange(todayInArgentina()).from
            ? []
            : [
                makeEvent({ id: 'old', modality: 'men', date: '2020-01-01' }),
                makeEvent({ id: 'new', modality: 'women', date: '2020-06-01' }),
              ],
      }),
    );
    renderWithProviders(<EventsHubPage />);
    const user = userEvent.setup();

    await screen.findByText(t.noUpcoming);
    await user.click(screen.getByRole('button', { name: t.past }));

    const items = await screen.findAllByRole('listitem');
    expect(items[0]).toHaveTextContent('Cancha abierta de pádelDamas');
    expect(items[1]).toHaveTextContent('Cancha abierta de pádelCaballeros');
    const events = pastEventsRange(todayInArgentina());
    expect(eventsApi.list).toHaveBeenCalledWith('c1', events.from, events.to, expect.anything());
    const tournaments = pastTournamentsRange(todayInArgentina());
    expect(tournamentsApi.list).toHaveBeenLastCalledWith('c1', tournaments.from, tournaments.to, expect.anything());
  });
});

describe('EventsHubPage: Crear evento asks the kind first', () => {
  it('opens the type picker, and Cancha abierta opens the event form', async () => {
    vi.mocked(eventsApi.list).mockResolvedValue({ events: [makeEvent({})] });
    renderWithProviders(<EventsHubPage />);
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: t.create }));
    const picker = await screen.findByRole('dialog', { name: ES_AR.events.typePicker.title });
    await user.click(within(picker).getByRole('button', { name: /Cancha abierta/ }));

    expect(await screen.findByRole('dialog', { name: t.create })).toHaveTextContent(t.createDescription);
    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: ES_AR.events.typePicker.title })).not.toBeInTheDocument();
    });
  });

  it('opens the tournament form when Torneo is picked', async () => {
    vi.mocked(eventsApi.list).mockResolvedValue({ events: [makeEvent({})] });
    renderWithProviders(<EventsHubPage />);
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: t.create }));
    const picker = await screen.findByRole('dialog', { name: ES_AR.events.typePicker.title });
    await user.click(within(picker).getByRole('button', { name: /Torneo/ }));

    expect(await screen.findByRole('dialog', { name: tt.form.title })).toBeInTheDocument();
  });
});

describe('EventsHubPage: detail of an open-play event', () => {
  it('opens the detail of the event named in ?event=, the way the grid block links here', async () => {
    vi.mocked(eventsApi.list).mockResolvedValue({ events: [makeEvent({})] });
    vi.mocked(eventsApi.get).mockResolvedValue({ event: makeEvent({}), participants: [] });
    renderWithProviders(<EventsHubPage />, { initialEntries: ['/events?event=ev1'] });

    expect(await screen.findByText(t.detail.noParticipants)).toBeInTheDocument();
    expect(eventsApi.get).toHaveBeenCalledWith('c1', 'ev1', expect.anything());
  });

  it('opens the detail when an event card is pressed', async () => {
    vi.mocked(eventsApi.list).mockResolvedValue({ events: [makeEvent({})] });
    vi.mocked(eventsApi.get).mockResolvedValue({ event: makeEvent({}), participants: [] });
    renderWithProviders(<EventsHubPage />);
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: /Cancha abierta de pádel/ }));

    await waitFor(() => {
      expect(eventsApi.get).toHaveBeenCalledWith('c1', 'ev1', expect.anything());
    });
  });
});
