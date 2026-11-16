import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderBooking } from '../../test/booking';
import { CAT_OPEN, TOURNAMENT } from '../../test/tournamentFixtures';
import type { PublicTournamentCategory, PublicTournamentSummary } from '../../api/types';
import { BOOKING_MESSAGES } from '../../messages';
import { agendaRows } from './agendaModel';
import { TournamentAgenda } from './TournamentAgenda';

const t = BOOKING_MESSAGES.tournaments;

/** Caballeros 8va: five places left, starts Sunday 6 at 18:00, with prizes. */
const men8: PublicTournamentCategory = {
  ...CAT_OPEN,
  id: 'c-men8',
  name: 'stored',
  modality: 'men',
  level: 8,
  suma: null,
  prizes: 'Trofeo y 2 pelotas por pareja',
  starts_on: '2099-12-06',
  starts_at: '18:00',
  spots_left: 5,
  price: 2_000_000,
};

/** Damas 5ta: two places left, starts Saturday 5 at 10:00. */
const women5: PublicTournamentCategory = {
  ...men8,
  id: 'c-women5',
  modality: 'women',
  level: 5,
  starts_on: '2099-12-05',
  starts_at: '10:00',
  spots_left: 2,
  prizes: null,
  price: 1_500_000,
};

function rowsOf(...categories: PublicTournamentCategory[]) {
  const tournament: PublicTournamentSummary = { ...TOURNAMENT, categories };
  return agendaRows([tournament]);
}

function renderAgenda(...categories: PublicTournamentCategory[]) {
  return renderBooking(<TournamentAgenda rows={rowsOf(...categories)} slug="test-club" />);
}

/** The agenda row whose title is this, found through its heading. */
function rowOf(title: string): HTMLElement {
  const article = screen.getByRole('heading', { name: title }).closest('article');
  if (!article) throw new Error(`no row for ${title}`);
  return article;
}

describe('TournamentAgenda rows', () => {
  it('lists each open category by its composed title under the sport, with a sign-up to that category', () => {
    renderAgenda(men8, women5);

    expect(screen.getByRole('heading', { level: 3, name: 'Caballeros 8va' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 3, name: 'Damas 5ta' })).toBeInTheDocument();
    expect(screen.getAllByText('Torneo de pádel')).toHaveLength(2);
    expect(screen.getByRole('link', { name: /Inscribirme.*Caballeros 8va/ })).toHaveAttribute(
      'href',
      '/c/test-club/tournaments/t1/inscripcion?category=c-men8',
    );
  });

  it('says the time, the places left and the price on one line, and no service fee', () => {
    renderAgenda(men8);

    expect(screen.getByText('18:00 hs')).toBeInTheDocument();
    expect(screen.getByText('Quedan 5 lugares')).toBeInTheDocument();
    expect(screen.getByText('$20.000 por pareja')).toBeInTheDocument();
    expect(screen.queryByText(/cargo de servicio/i)).not.toBeInTheDocument();
  });

  it('says the last places, highlighted, when two or fewer remain', () => {
    renderAgenda(women5);

    expect(screen.getByText(t.lastSpots)).toHaveClass('text-warning-text');
    expect(screen.queryByText(/Quedan/)).not.toBeInTheDocument();
  });

  it('never shows the stored tournament name or a venue', () => {
    renderAgenda(men8);

    expect(screen.queryByText(TOURNAMENT.name)).not.toBeInTheDocument();
    expect(screen.queryByText('Club Norte')).not.toBeInTheDocument();
  });

  it('writes a category that predates its start by its stored name and with no time', () => {
    renderAgenda(CAT_OPEN);

    expect(screen.getByRole('heading', { level: 3, name: '8va' })).toBeInTheDocument();
    expect(screen.queryByText(/hs$/)).not.toBeInTheDocument();
  });

  it('says so when no category matches the filters', async () => {
    const user = userEvent.setup();
    renderAgenda(men8);

    await user.click(screen.getByRole('button', { name: 'Damas' }));

    expect(screen.getByText(t.noAgendaRows)).toBeInTheDocument();
  });

  it('states that it lists only the categories with a place', () => {
    renderAgenda(men8);

    expect(screen.getByText(t.onlyWithSpots)).toBeInTheDocument();
  });
});

describe('TournamentAgenda filters', () => {
  it('keeps the rows of one modality, and a category without one only under Todas', async () => {
    const user = userEvent.setup();
    renderAgenda(men8, women5, CAT_OPEN);

    await user.click(screen.getByRole('button', { name: 'Damas' }));
    expect(screen.getByRole('button', { name: 'Damas' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('heading', { name: 'Damas 5ta' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Caballeros 8va' })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: '8va' })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: t.filterAll }));
    expect(screen.getByRole('heading', { name: 'Caballeros 8va' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '8va' })).toBeInTheDocument();
  });

  it('keeps the rows of one day, and Todos clears it', async () => {
    const user = userEvent.setup();
    renderAgenda(men8, women5);

    await user.click(screen.getByRole('button', { name: 'Dom 6' }));
    expect(screen.getByRole('heading', { name: 'Caballeros 8va' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Damas 5ta' })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: t.filterAllDays }));
    expect(screen.getByRole('heading', { name: 'Damas 5ta' })).toBeInTheDocument();
  });
});

describe('TournamentAgenda detail', () => {
  it('opens the category with its day and time, price, prizes and format, and closes it', async () => {
    const user = userEvent.setup();
    renderAgenda(men8);

    await user.click(within(rowOf('Caballeros 8va')).getByRole('button', { name: t.viewDetail }));

    const dialog = await screen.findByRole('alertdialog');
    expect(within(dialog).getByRole('heading', { name: 'Caballeros 8va' })).toBeInTheDocument();
    expect(within(dialog).getByText('Domingo 06/12 · 18:00 hs')).toBeInTheDocument();
    expect(within(dialog).getByText(t.inscriptionLabel)).toBeInTheDocument();
    expect(within(dialog).getByText('$20.000 por pareja')).toBeInTheDocument();
    expect(within(dialog).getByText(t.prizesLabel)).toBeInTheDocument();
    expect(within(dialog).getByText('Trofeo y 2 pelotas por pareja')).toBeInTheDocument();
    expect(within(dialog).getByText(`${t.formatGroups} · ${t.zonesOf} 3 ${t.pairsPassed} 2`)).toBeInTheDocument();
    expect(within(dialog).getByRole('link', { name: t.signUpCategory })).toHaveAttribute(
      'href',
      '/c/test-club/tournaments/t1/inscripcion?category=c-men8',
    );

    await user.click(within(dialog).getByRole('button', { name: t.closeDetail }));
    await waitFor(() => {
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
    });
  });

  it('leaves out the prizes when the category has none', async () => {
    const user = userEvent.setup();
    renderAgenda(women5);

    await user.click(within(rowOf('Damas 5ta')).getByRole('button', { name: t.viewDetail }));

    const dialog = await screen.findByRole('alertdialog');
    expect(within(dialog).queryByText(t.prizesLabel)).not.toBeInTheDocument();
  });
});
