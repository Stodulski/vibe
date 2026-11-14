import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SeriesConflictsPanel } from './SeriesConflictsPanel';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { BookingSeriesConflict, CourtAlternative } from '@/shared/types/api.types';

const t = ES_AR.bookings.series;

// Three Mondays held by a booking, a block and an event.
const MONDAYS: BookingSeriesConflict[] = [
  { date: '2026-12-07', reason: 'overlap', held_by: 'booking' },
  { date: '2026-12-21', reason: 'blocked' },
  { date: '2027-01-04', reason: 'overlap', held_by: 'event' },
];

const plain = (text: string | null) => (text ?? '').replace(/\u00A0/g, ' ');

/** Matches an accessible name ignoring the non-breaking spaces `keepTogether` puts in court names. */
const named = (name: string) => (accessible: string) => plain(accessible) === name;

function renderPanel({
  endless = false,
  alternatives = [],
  conflicts = MONDAYS,
}: {
  endless?: boolean;
  alternatives?: CourtAlternative[];
  conflicts?: BookingSeriesConflict[];
} = {}) {
  const onSkip = vi.fn();
  const onBack = vi.fn();
  const onUseCourt = vi.fn();
  render(
    <SeriesConflictsPanel
      courtName="Cancha 1"
      conflicts={conflicts}
      endless={endless}
      pending={false}
      alternatives={alternatives}
      onUseCourt={onUseCourt}
      onSkip={onSkip}
      onBack={onBack}
    />,
  );
  return { onSkip, onBack, onUseCourt };
}

const FREE_AND_BUSY: CourtAlternative[] = [
  { court_id: 'ct2', court_name: 'Cancha 2', conflicts: 0 },
  { court_id: 'ct3', court_name: 'Cancha 3', conflicts: 2 },
];

describe('SeriesConflictsPanel, the problem', () => {
  it('states it with the court and the weekday, and the two ways out', () => {
    renderPanel({ alternatives: FREE_AND_BUSY });

    expect(screen.getByRole('heading', { name: named('Cancha 1 está ocupada 3 lunes') })).toBeInTheDocument();
    expect(
      screen.getByText('Elegí otra cancha libre todas las semanas, o creá el turno fijo en Cancha 1 sin esos 3 días.'),
    ).toBeInTheDocument();
  });

  it('uses the singular for a single date', () => {
    renderPanel({ conflicts: MONDAYS.slice(0, 1) });

    expect(screen.getByRole('heading', { name: named('Cancha 1 está ocupada 1 lunes') })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Crear sin ese día' })).toBeInTheDocument();
  });

  it('does not suggest another free court when the server offers none', () => {
    renderPanel();

    expect(
      screen.getByText('Creá el turno fijo en Cancha 1 sin esos 3 días, o volvé y cambiá el horario.'),
    ).toBeInTheDocument();
  });

  it('says a series with no end was checked for the next year only, and not otherwise', () => {
    renderPanel({ endless: true });

    expect(screen.getByText(t.conflictsEndless)).toBeInTheDocument();
  });

  it('does not talk about the next year for a series that ends', () => {
    renderPanel({ endless: false });

    expect(screen.queryByText(t.conflictsEndless)).not.toBeInTheDocument();
  });
});

describe('SeriesConflictsPanel, staying on the court', () => {
  it('lists each date in full, with the year, and what holds it', () => {
    renderPanel();

    const option = screen.getByRole('region', { name: named('Quedarte en Cancha 1') });
    expect(within(option).getByText('Estos días no se reserva:')).toBeInTheDocument();
    const items = within(option)
      .getAllByRole('listitem')
      .map((li) => li.textContent);
    expect(items).toEqual([
      'Lunes 7 de diciembre de 2026Reserva',
      'Lunes 21 de diciembre de 2026Bloqueo',
      'Lunes 4 de enero de 2027Evento',
    ]);
  });

  it('names every kind of reason in a short tag', () => {
    expect(t.conflictTags).toEqual({
      booking: 'Reserva',
      fixed: 'Otro turno fijo',
      event: 'Evento',
      match: 'Partido de torneo',
      overlap: 'Ocupada',
      blocked: 'Bloqueo',
      cancelled_by_owner: 'Liberada',
      closed_day: 'Complejo cerrado',
      no_price: 'Sin precio',
      client_blocked: 'Cliente bloqueado',
      missed: 'No se pudo crear',
    });
  });

  it('creates without those days, or goes back', async () => {
    const user = userEvent.setup();
    const { onSkip, onBack } = renderPanel();

    await user.click(screen.getByRole('button', { name: 'Crear sin esos 3 días' }));
    await user.click(screen.getByRole('button', { name: 'Volver' }));

    expect(onSkip).toHaveBeenCalledOnce();
    expect(onBack).toHaveBeenCalledOnce();
  });
});

describe('SeriesConflictsPanel, other courts', () => {
  it('offers each court with how free it is, and switches to the one picked', async () => {
    const user = userEvent.setup();
    const { onUseCourt } = renderPanel({ alternatives: FREE_AND_BUSY });

    const option = screen.getByRole('region', { name: 'Cambiar de cancha' });
    const rows = within(option)
      .getAllByTestId('court-alternative')
      .map((row) => plain(row.textContent));
    expect(rows).toEqual(['Cancha 2Libre todos los lunesUsar Cancha 2', 'Cancha 3Ocupada 2 lunesUsar Cancha 3']);

    await user.click(screen.getByRole('button', { name: named('Usar Cancha 3') }));

    expect(onUseCourt).toHaveBeenCalledWith('ct3');
  });

  it('writes "Ocupada 1 lunes" in the singular', () => {
    renderPanel({ alternatives: [{ court_id: 'ct3', court_name: 'Cancha 3', conflicts: 1 }] });

    expect(plain(screen.getByTestId('court-alternative').textContent)).toBe('Cancha 3Ocupada 1 lunesUsar Cancha 3');
    expect(
      screen.getByText('Elegí otra cancha con menos días ocupados, o creá el turno fijo en Cancha 1 sin esos 3 días.'),
    ).toBeInTheDocument();
  });

  it('shows no court option when the server offers none', () => {
    renderPanel();

    expect(screen.queryByRole('region', { name: 'Cambiar de cancha' })).not.toBeInTheDocument();
    expect(screen.queryByTestId('court-alternative')).not.toBeInTheDocument();
  });
});
