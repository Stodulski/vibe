import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { PublicOpenPlayEvent, PublicTournamentCategory, PublicTournamentSummary } from '../../api/types';
import { renderBooking } from '../../test/booking';
import { EVENT } from '../../test/eventFixtures';
import { CAT_OPEN, TOURNAMENT } from '../../test/tournamentFixtures';
import { BOOKING_MESSAGES } from '../../messages';
import { agendaRows } from '../tournaments/agendaModel';
import { EventsAgenda } from './EventsAgenda';
import { eventRows, type EventRow } from './eventRows';
import { openPlayRows } from './openPlayAgendaModel';

const t = BOOKING_MESSAGES.events;

/**
 * Every observer the agenda has created, newest last. A test plays the sentinel
 * entering the viewport by firing the newest one that is still connected.
 */
const observers: FakeIntersectionObserver[] = [];

class FakeIntersectionObserver {
  connected = true;
  readonly root = null;
  readonly rootMargin = '';
  readonly scrollMargin = '';
  readonly thresholds: number[] = [];
  private readonly callback: IntersectionObserverCallback;

  constructor(callback: IntersectionObserverCallback) {
    this.callback = callback;
    observers.push(this);
  }

  observe() {
    // The fake fires its callback by hand, so nothing is watched for real.
  }

  unobserve() {
    // Nothing to stop: see observe.
  }

  disconnect() {
    this.connected = false;
  }

  takeRecords(): IntersectionObserverEntry[] {
    return [];
  }

  enter() {
    this.callback([{ isIntersecting: true } as IntersectionObserverEntry], this);
  }
}

beforeEach(() => {
  observers.length = 0;
  vi.stubGlobal('IntersectionObserver', FakeIntersectionObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

/** The sentinel after the last shown row enters the viewport. */
function sentinelEnters() {
  const watching = observers.filter((observer) => observer.connected);
  const newest = watching[watching.length - 1];
  if (!newest) throw new Error('no observer is watching the sentinel');
  act(() => {
    newest.enter();
  });
}

/** Whether the agenda is watching a sentinel for more rows. */
function isWatchingSentinel(): boolean {
  return observers.some((observer) => observer.connected);
}

/** A pádel category open for sign-up with five places left. */
function category(
  id: string,
  date: string,
  time: string | null,
  modality: 'men' | 'women',
  level: number,
): PublicTournamentCategory {
  return {
    ...CAT_OPEN,
    id,
    modality,
    level,
    suma: null,
    prizes: null,
    price: 2_000_000,
    status: 'open',
    spots_left: 5,
    starts_on: date,
    starts_at: time,
  };
}

function tournamentRows(...categories: PublicTournamentCategory[]) {
  const tournament: PublicTournamentSummary = { ...TOURNAMENT, categories };
  return agendaRows([tournament]);
}

/** Twenty-five open plays, one a day in December 2099: fifteen for Caballeros, ten for Damas. */
function manyOpenPlays(): PublicOpenPlayEvent[] {
  return Array.from({ length: 25 }, (_, index) => ({
    ...EVENT,
    id: `ev-${String(index)}`,
    date: `2099-12-${String(index + 1).padStart(2, '0')}`,
    modality: index < 15 ? 'men' : 'women',
  }));
}

function renderAgenda(rows: EventRow[]) {
  return renderBooking(<EventsAgenda rows={rows} slug="test-club" />);
}

/** The agenda row whose title is this, found through its heading. */
function rowOf(title: string): HTMLElement {
  const article = screen.getByRole('heading', { name: title }).closest('article');
  if (!article) throw new Error(`no row for ${title}`);
  return article;
}

function modalityFilter(): HTMLElement {
  return screen.getByRole('combobox', { name: t.filterModality });
}

function dayFilter(): HTMLElement {
  return screen.getByRole('combobox', { name: t.filterDay });
}

describe('EventsAgenda list', () => {
  it('lists open play and tournaments in one list, soonest first, each under its own label', () => {
    renderAgenda(
      eventRows(
        openPlayRows([EVENT]),
        tournamentRows(category('t-men8', '2099-12-06', '18:00', 'men', 8)),
      ),
    );

    const articles = screen.getAllByRole('article');
    expect(articles[0]).toBe(rowOf('Abierta a todos'));
    expect(articles[1]).toBe(rowOf('Caballeros 8va'));
    expect(within(rowOf('Abierta a todos')).getByText('Cancha abierta de pádel')).toBeInTheDocument();
    expect(within(rowOf('Caballeros 8va')).getByText('Torneo de pádel')).toBeInTheDocument();
  });

  it('filters the merged list by modality and by day, across both kinds', async () => {
    const user = userEvent.setup();
    renderAgenda(
      eventRows(
        openPlayRows([EVENT, { ...EVENT, id: 'ev-men', start_time: '20:00', end_time: '23:00', modality: 'men' }]),
        tournamentRows(category('t-women5', '2099-12-06', '10:00', 'women', 5)),
      ),
    );

    await user.selectOptions(modalityFilter(), 'Damas');
    expect(modalityFilter()).toHaveValue('women');
    expect(screen.getByRole('heading', { name: 'Damas 5ta' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Abierta a todos' })).not.toBeInTheDocument();
    expect(screen.getAllByRole('article')).toHaveLength(1);

    await user.selectOptions(modalityFilter(), t.filterAll);
    await user.selectOptions(dayFilter(), 'Dom 6 dic');
    expect(screen.getAllByRole('article')).toHaveLength(1);
    expect(screen.getByRole('heading', { name: 'Damas 5ta' })).toBeInTheDocument();

    await user.selectOptions(dayFilter(), t.filterAllDays);
    expect(screen.getAllByRole('article')).toHaveLength(3);
  });

  it('says so when no row matches the filters', async () => {
    const user = userEvent.setup();
    renderAgenda(eventRows(openPlayRows([EVENT]), tournamentRows(category('t-women5', '2099-12-06', '10:00', 'women', 5))));

    await user.selectOptions(modalityFilter(), 'Damas');
    await user.selectOptions(dayFilter(), 'Sáb 5 dic');

    expect(screen.getByText(t.noAgendaRows)).toBeInTheDocument();
  });

  it('names the modality options of both kinds in the words of their sport', () => {
    renderAgenda(
      eventRows(
        openPlayRows([EVENT]),
        tournamentRows(category('t-women5', '2099-12-06', '10:00', 'women', 5)),
      ),
    );

    expect(within(modalityFilter()).getByRole('option', { name: 'Damas' })).toBeInTheDocument();
    expect(within(modalityFilter()).queryByRole('option', { name: 'Caballeros' })).not.toBeInTheDocument();
  });

});

describe('EventsAgenda detail', () => {
  it('opens an open-play row with its day and time, price, courts, who may play and the sign-up', async () => {
    const user = userEvent.setup();
    renderAgenda(eventRows(openPlayRows([EVENT]), []));

    await user.click(within(rowOf('Abierta a todos')).getByRole('button', { name: t.viewDetail }));

    const dialog = await screen.findByRole('alertdialog');
    expect(within(dialog).getByRole('heading', { name: 'Cancha abierta de pádel' })).toBeInTheDocument();
    expect(within(dialog).getByText('Sábado 05/12 · 19:00 a 22:00')).toBeInTheDocument();
    expect(within(dialog).getByText(t.detailPriceLabel)).toBeInTheDocument();
    expect(within(dialog).getByText('$10.000 por persona')).toBeInTheDocument();
    expect(within(dialog).getByText('Cancha 1, Cancha 2')).toBeInTheDocument();
    expect(within(dialog).getByText(t.detailRequirementLabel)).toBeInTheDocument();
    expect(within(dialog).queryByText(/cargo de servicio/i)).not.toBeInTheDocument();
    expect(within(dialog).getByRole('link', { name: t.signUp })).toHaveAttribute('href', '/c/test-club/events/ev1');

    await user.click(within(dialog).getByRole('button', { name: t.closeDetail }));
    await waitFor(() => {
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
    });
  });

  it('opens a tournament row with its day and time, price and the sign-up to its category', async () => {
    const user = userEvent.setup();
    renderAgenda(eventRows([], tournamentRows(category('c-men8', '2099-12-06', '18:00', 'men', 8))));

    await user.click(within(rowOf('Caballeros 8va')).getByRole('button', { name: BOOKING_MESSAGES.tournaments.viewDetail }));

    const dialog = await screen.findByRole('alertdialog');
    expect(within(dialog).getByRole('heading', { name: 'Caballeros 8va' })).toBeInTheDocument();
    expect(within(dialog).getByText('Domingo 06/12 · 18:00 hs')).toBeInTheDocument();
    expect(within(dialog).getByText('$20.000 por pareja')).toBeInTheDocument();
    expect(within(dialog).getByRole('link', { name: BOOKING_MESSAGES.tournaments.signUpCategory })).toHaveAttribute(
      'href',
      '/c/test-club/tournaments/t1/inscripcion?category=c-men8',
    );
  });
});

describe('EventsAgenda infinite scroll', () => {
  it('shows ten rows, then ten more each time the sentinel enters the view', () => {
    renderAgenda(eventRows(openPlayRows(manyOpenPlays()), []));

    expect(screen.getAllByRole('article')).toHaveLength(10);
    sentinelEnters();
    expect(screen.getAllByRole('article')).toHaveLength(20);
    sentinelEnters();
    expect(screen.getAllByRole('article')).toHaveLength(25);
  });

  it('stops watching the sentinel once every row is shown', () => {
    renderAgenda(eventRows(openPlayRows(manyOpenPlays()), []));

    sentinelEnters();
    sentinelEnters();

    expect(screen.getAllByRole('article')).toHaveLength(25);
    expect(isWatchingSentinel()).toBe(false);
  });

  it('watches no sentinel for a list that fits on one screen', () => {
    renderAgenda(eventRows(openPlayRows([EVENT]), []));

    expect(isWatchingSentinel()).toBe(false);
  });

  it('goes back to the first ten rows when a filter changes', async () => {
    const user = userEvent.setup();
    renderAgenda(eventRows(openPlayRows(manyOpenPlays()), []));

    sentinelEnters();
    expect(screen.getAllByRole('article')).toHaveLength(20);

    await user.selectOptions(modalityFilter(), 'Caballeros');
    expect(screen.getAllByRole('article')).toHaveLength(10);
    expect(isWatchingSentinel()).toBe(true);

    sentinelEnters();
    expect(screen.getAllByRole('article')).toHaveLength(15);
    expect(isWatchingSentinel()).toBe(false);
  });
});
