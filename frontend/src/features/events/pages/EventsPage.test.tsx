import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { todayInArgentina } from '@/shared/lib/today';
import { pastRange, upcomingRange } from '../lib/eventRanges';
import EventsPage from './EventsPage';
import type { OpenPlayEvent } from '@/shared/types/api.types';

vi.mock('../api/events.api', () => ({ eventsApi: { list: vi.fn(), get: vi.fn(), cancel: vi.fn(), create: vi.fn() } }));
vi.mock('@/features/complex', () => ({
  useSelectedComplex: () => ({ selectedComplexId: 'c1' }),
}));
vi.mock('@/features/courts', () => ({ useCourts: () => ({ data: [] }) }));

const t = ES_AR.events;

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

describe('EventsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('lists the upcoming events of the complex: name, courts, spots taken and price', async () => {
    const { eventsApi } = await import('../api/events.api').then((m) => ({ eventsApi: m.eventsApi }));
    vi.mocked(eventsApi.list).mockResolvedValue({ events: [makeEvent({})] });
    renderWithProviders(<EventsPage />);

    const card = await screen.findByRole('button', { name: /Americano de viernes/ });
    expect(card).toHaveTextContent('Cancha 1');
    expect(card).toHaveTextContent(t.spotsTaken(5, 20));
    const { from, to } = upcomingRange(todayInArgentina());
    expect(eventsApi.list).toHaveBeenCalledWith('c1', from, to, expect.anything());
  });

  it('marks a cancelled event and a full one', async () => {
    const { eventsApi } = await import('../api/events.api').then((m) => ({ eventsApi: m.eventsApi }));
    vi.mocked(eventsApi.list).mockResolvedValue({
      events: [
        makeEvent({ id: 'a', name: 'Cancelado', status: 'cancelled' }),
        makeEvent({ id: 'b', name: 'Lleno', spots_left: 0, confirmed_count: 20 }),
      ],
    });
    renderWithProviders(<EventsPage />);

    expect(await screen.findByRole('button', { name: /Cancelado/ })).toHaveTextContent(t.cancelledBadge);
    expect(screen.getByRole('button', { name: /Lleno/ })).toHaveTextContent(t.fullBadge);
  });

  it('invites creating the first event when there are none, and opens the create dialog', async () => {
    const { eventsApi } = await import('../api/events.api').then((m) => ({ eventsApi: m.eventsApi }));
    vi.mocked(eventsApi.list).mockResolvedValue({ events: [] });
    renderWithProviders(<EventsPage />);
    const user = userEvent.setup();

    expect(await screen.findByText(t.noUpcoming)).toBeInTheDocument();
    await user.click(
      within(screen.getByRole('region', { name: t.noUpcoming })).getByRole('button', { name: t.create }),
    );

    expect(await screen.findByRole('dialog')).toHaveTextContent(t.createDescription);
  });
});

describe('EventsPage tabs and detail', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows the past year, most recent first, once its tab is opened', async () => {
    const { eventsApi } = await import('../api/events.api').then((m) => ({ eventsApi: m.eventsApi }));
    vi.mocked(eventsApi.list).mockImplementation((_id, from) =>
      Promise.resolve({
        events:
          from === upcomingRange(todayInArgentina()).from
            ? []
            : [
                makeEvent({ id: 'old', name: 'Viejo', date: '2020-01-01' }),
                makeEvent({ id: 'new', name: 'Reciente', date: '2020-06-01' }),
              ],
      }),
    );
    renderWithProviders(<EventsPage />);
    const user = userEvent.setup();

    await screen.findByText(t.noUpcoming);
    await user.click(screen.getByRole('button', { name: t.past }));

    const items = await screen.findAllByRole('listitem');
    expect(items[0]).toHaveTextContent('Reciente');
    expect(items[1]).toHaveTextContent('Viejo');
    const { from, to } = pastRange(todayInArgentina());
    expect(eventsApi.list).toHaveBeenCalledWith('c1', from, to, expect.anything());
  });

  it('opens the detail of the event named in ?event=, the way the grid block links here', async () => {
    const { eventsApi } = await import('../api/events.api').then((m) => ({ eventsApi: m.eventsApi }));
    vi.mocked(eventsApi.list).mockResolvedValue({ events: [makeEvent({})] });
    vi.mocked(eventsApi.get).mockResolvedValue({ event: makeEvent({}), participants: [] });
    renderWithProviders(<EventsPage />, { initialEntries: ['/open-play?event=ev1'] });

    expect(await screen.findByText(t.detail.noParticipants)).toBeInTheDocument();
    expect(eventsApi.get).toHaveBeenCalledWith('c1', 'ev1', expect.anything());
  });

  it('opens the detail when a card is pressed', async () => {
    const { eventsApi } = await import('../api/events.api').then((m) => ({ eventsApi: m.eventsApi }));
    vi.mocked(eventsApi.list).mockResolvedValue({ events: [makeEvent({})] });
    vi.mocked(eventsApi.get).mockResolvedValue({ event: makeEvent({}), participants: [] });
    renderWithProviders(<EventsPage />);
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: /Americano de viernes/ }));

    await waitFor(() => {
      expect(eventsApi.get).toHaveBeenCalledWith('c1', 'ev1', expect.anything());
    });
  });
});
