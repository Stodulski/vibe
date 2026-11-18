import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../test/msw/server';
import type { PublicOpenPlayEvent, PublicTournamentCategory, PublicTournamentSummary } from '../../api/types';
import { renderBooking } from '../../test/booking';
import { chooseOption } from '../../test/chooseOption';
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

  it('filters the merged list by modality, across both kinds', async () => {
    const user = userEvent.setup();
    renderAgenda(
      eventRows(
        openPlayRows([EVENT, { ...EVENT, id: 'ev-men', start_time: '20:00', end_time: '23:00', modality: 'men' }]),
        tournamentRows(category('t-women5', '2099-12-06', '10:00', 'women', 5)),
      ),
    );

    await chooseOption(user, modalityFilter(), 'Damas');
    expect(modalityFilter()).toHaveTextContent('Damas');
    expect(screen.getByRole('heading', { name: 'Damas 5ta' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Abierta a todos' })).not.toBeInTheDocument();
    expect(screen.getAllByRole('article')).toHaveLength(1);

    await chooseOption(user, modalityFilter(), t.filterAll);
    expect(screen.getAllByRole('article')).toHaveLength(3);
  });

  it('offers no day filter: the list is already in date order', () => {
    renderAgenda(eventRows(openPlayRows([EVENT]), tournamentRows(category('t-women5', '2099-12-06', '10:00', 'women', 5))));

    expect(screen.queryByRole('combobox', { name: t.filterDay })).not.toBeInTheDocument();
    expect(screen.getAllByRole('combobox')).toHaveLength(1);
  });

  it('names the modality options of both kinds in the words of their sport', async () => {
    const user = userEvent.setup();
    renderAgenda(
      eventRows(
        openPlayRows([EVENT]),
        tournamentRows(category('t-women5', '2099-12-06', '10:00', 'women', 5)),
      ),
    );

    await user.click(modalityFilter());

    expect(await screen.findByRole('option', { name: 'Damas' })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'Caballeros' })).not.toBeInTheDocument();
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

    await chooseOption(user, modalityFilter(),'Caballeros');
    expect(screen.getAllByRole('article')).toHaveLength(10);
    expect(isWatchingSentinel()).toBe(true);

    sentinelEnters();
    expect(screen.getAllByRole('article')).toHaveLength(15);
    expect(isWatchingSentinel()).toBe(false);
  });
});

describe('EventsAgenda waitlist', () => {
  const w = BOOKING_MESSAGES.waitlist;

  it('offers "Avisarme" instead of the sign-up on a full event, and saves the email for that event', async () => {
    const user = userEvent.setup();
    let body: unknown = null;
    server.use(
      http.post('*/public/complexes/:slug/waitlist/places', async ({ request }) => {
        body = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );
    renderAgenda(eventRows(openPlayRows([{ ...EVENT, spots_left: 0 }]), []));

    expect(screen.queryByRole('link', { name: /Inscribirme/ })).not.toBeInTheDocument();
    expect(screen.getByText(w.full)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: new RegExp(w.notifyMe) }));

    const dialog = await screen.findByRole('alertdialog');
    await user.type(within(dialog).getByLabelText(w.emailLabel), ' Ana@Example.com ');
    await user.click(within(dialog).getByRole('button', { name: w.send }));

    expect(await within(dialog).findByText(w.done)).toBeInTheDocument();
    expect(body).toEqual({ kind: 'open_play', event_id: EVENT.id, email: 'Ana@Example.com' });
  });

  it('asks for a valid email before sending', async () => {
    const user = userEvent.setup();
    renderAgenda(eventRows(openPlayRows([{ ...EVENT, spots_left: 0 }]), []));

    await user.click(screen.getByRole('button', { name: new RegExp(w.notifyMe) }));
    const dialog = await screen.findByRole('alertdialog');
    await user.type(within(dialog).getByLabelText(w.emailLabel), 'nope');
    await user.click(within(dialog).getByRole('button', { name: w.send }));

    expect(within(dialog).getByText(w.emailInvalid)).toBeInTheDocument();
  });

  it('sends the category of a full tournament row', async () => {
    const user = userEvent.setup();
    let body: unknown = null;
    server.use(
      http.post('*/public/complexes/:slug/waitlist/places', async ({ request }) => {
        body = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const full = { ...CAT_OPEN, spots_left: 0 };
    renderAgenda(eventRows([], agendaRows([{ ...TOURNAMENT, categories: [full] }])));

    await user.click(screen.getByRole('button', { name: new RegExp(w.notifyMe) }));
    const dialog = await screen.findByRole('alertdialog');
    await user.type(within(dialog).getByLabelText(w.emailLabel), 'ana@example.com');
    await user.click(within(dialog).getByRole('button', { name: w.send }));

    await within(dialog).findByText(w.done);
    expect(body).toEqual({ kind: 'tournament_category', category_id: CAT_OPEN.id, email: 'ana@example.com' });
  });
});
