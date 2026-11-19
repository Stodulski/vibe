import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SeriesConflictsPanel } from './SeriesConflictsPanel';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { BookingSeriesConflict } from '@/shared/types/api.types';

const t = ES_AR.bookings.series;

// Three Mondays held by a booking, a block and an event. Cancha 2 is free on the
// first, Cancha 2 and 3 on the second, no court on the third.
const MONDAYS: BookingSeriesConflict[] = [
  {
    date: '2026-12-07',
    reason: 'overlap',
    held_by: 'booking',
    free_courts: [{ court_id: 'ct2', court_name: 'Cancha 2' }],
  },
  {
    date: '2026-12-21',
    reason: 'blocked',
    free_courts: [
      { court_id: 'ct2', court_name: 'Cancha 2' },
      { court_id: 'ct3', court_name: 'Cancha 3' },
    ],
  },
  { date: '2027-01-04', reason: 'overlap', held_by: 'event', free_courts: [] },
];

const plain = (text: string | null) => (text ?? '').replace(/\u00A0/g, ' ');

/** Matches an accessible name ignoring the non-breaking spaces `keepTogether` puts in court names. */
const named = (name: string) => (accessible: string) => plain(accessible) === name;

function renderPanel({
  endless = false,
  conflicts = MONDAYS,
}: { endless?: boolean; conflicts?: BookingSeriesConflict[] } = {}) {
  const onCreate = vi.fn();
  const onBack = vi.fn();
  render(
    <SeriesConflictsPanel
      courtName="Cancha 1"
      conflicts={conflicts}
      endless={endless}
      pending={false}
      onCreate={onCreate}
      onBack={onBack}
    />,
  );
  return { onCreate, onBack };
}

/** Picks one day's option by name, opening that day's selector first. */
async function pick(user: ReturnType<typeof userEvent.setup>, trigger: HTMLElement, option: string) {
  await user.click(trigger);
  await user.click(await screen.findByRole('option', { name: named(option) }));
}

/** The options one day's selector offers: it is opened to read them, then closed again. */
async function optionsOf(user: ReturnType<typeof userEvent.setup>, trigger: HTMLElement): Promise<string[]> {
  await user.click(trigger);
  const names = (await screen.findAllByRole('option')).map((option) => plain(option.textContent));
  await user.keyboard('{Escape}');
  return names;
}

describe('SeriesConflictsPanel, the problem', () => {
  it('states it with the court and the weekday', () => {
    renderPanel();

    expect(screen.getByRole('heading', { name: named('Cancha 1 está ocupada 3 lunes') })).toBeInTheDocument();
  });

  it('uses the singular for a single date', () => {
    renderPanel({ conflicts: MONDAYS.slice(0, 1) });

    expect(screen.getByRole('heading', { name: named('Cancha 1 está ocupada 1 lunes') })).toBeInTheDocument();
  });

  it('says a series with no end was checked for the next year only', () => {
    renderPanel({ endless: true });

    expect(screen.getByText(t.conflictsEndless)).toBeInTheDocument();
  });

  it('does not talk about the next year for a series that ends', () => {
    renderPanel({ endless: false });

    expect(screen.queryByText(t.conflictsEndless)).not.toBeInTheDocument();
  });
});

describe('SeriesConflictsPanel, the occupied dates', () => {
  it('lists each date in full, with the year, and what holds it', () => {
    renderPanel();

    const list = screen.getByRole('list', { name: named('Días ocupados en Cancha 1') });
    const rows = within(list).getAllByRole('listitem');
    expect(rows.map((row) => plain(within(row).getByTestId('conflict-date').textContent))).toEqual([
      'Lunes 7 de diciembre de 2026',
      'Lunes 21 de diciembre de 2026',
      'Lunes 4 de enero de 2027',
    ]);
    expect(rows.map((row) => within(row).getByTestId('conflict-tag').textContent)).toEqual([
      'Reserva',
      'Bloqueo',
      'Evento',
    ]);
  });
});

describe('SeriesConflictsPanel, what to do each day', () => {
  it('defaults every day to "No reservar ese día", offering only the courts free that day', async () => {
    const user = userEvent.setup();
    renderPanel();

    const first = screen.getByRole('combobox', { name: named('Ese día: Lunes 7 de diciembre de 2026') });
    const second = screen.getByRole('combobox', { name: named('Ese día: Lunes 21 de diciembre de 2026') });
    expect(first).toHaveTextContent('No reservar ese día');
    expect(second).toHaveTextContent('No reservar ese día');
    expect(await optionsOf(user, first)).toEqual(['No reservar ese día', 'Cancha 2']);
    expect(await optionsOf(user, second)).toEqual(['No reservar ese día', 'Cancha 2', 'Cancha 3']);
  });

  it('says so, with no selector, on a day no other court is free', () => {
    renderPanel();

    const row = screen.getByText('No hay otra cancha abierta ese día').closest('li');
    expect(row).not.toBeNull();
    expect(within(row as HTMLElement).queryByRole('combobox')).not.toBeInTheDocument();
    expect(screen.getAllByRole('combobox')).toHaveLength(2);
  });
});

describe('SeriesConflictsPanel, creating', () => {
  it('creates with the days picked for another court, and none for the days left alone', async () => {
    const user = userEvent.setup();
    const { onCreate } = renderPanel();

    await pick(
      user,
      screen.getByRole('combobox', { name: named('Ese día: Lunes 21 de diciembre de 2026') }),
      'Cancha 3',
    );
    await user.click(screen.getByRole('button', { name: 'Crear turno fijo' }));

    expect(onCreate).toHaveBeenCalledExactlyOnceWith([{ date: '2026-12-21', court_id: 'ct3' }]);
  });

  it('creates with no date_courts when every day stays unbooked', async () => {
    const user = userEvent.setup();
    const { onCreate } = renderPanel();

    await user.click(screen.getByRole('button', { name: 'Crear turno fijo' }));

    expect(onCreate).toHaveBeenCalledExactlyOnceWith([]);
  });

  it('puts a day back on "No reservar ese día"', async () => {
    const user = userEvent.setup();
    const { onCreate } = renderPanel();
    const select = screen.getByRole('combobox', { name: named('Ese día: Lunes 7 de diciembre de 2026') });

    await pick(user, select, 'Cancha 2');
    await pick(user, select, 'No reservar ese día');
    await user.click(screen.getByRole('button', { name: 'Crear turno fijo' }));

    expect(onCreate).toHaveBeenCalledExactlyOnceWith([]);
  });

  it('counts what will happen above the buttons', async () => {
    const user = userEvent.setup();
    renderPanel();

    expect(screen.getByText('3 días sin reservar')).toBeInTheDocument();

    await pick(
      user,
      screen.getByRole('combobox', { name: named('Ese día: Lunes 7 de diciembre de 2026') }),
      'Cancha 2',
    );
    expect(screen.getByText('1 día en otra cancha, 2 días sin reservar')).toBeInTheDocument();

    await pick(
      user,
      screen.getByRole('combobox', { name: named('Ese día: Lunes 21 de diciembre de 2026') }),
      'Cancha 3',
    );
    expect(screen.getByText('2 días en otra cancha, 1 día sin reservar')).toBeInTheDocument();
  });

  it('goes back', async () => {
    const user = userEvent.setup();
    const { onBack } = renderPanel();

    await user.click(screen.getByRole('button', { name: 'Volver' }));

    expect(onBack).toHaveBeenCalledOnce();
  });
});

describe('SeriesConflictsPanel, a long list', () => {
  it('lists every occupied date, however many there are', () => {
    const many: BookingSeriesConflict[] = Array.from({ length: 8 }, (_, i) => ({
      date: `2026-12-${String(7 + i * 7).padStart(2, '0')}`.replace('2026-12-35', '2027-01-04'),
      reason: 'overlap' as const,
      held_by: 'booking' as const,
      free_courts: [],
    }));
    renderPanel({ conflicts: many });

    const list = screen.getByRole('list', { name: /ocupad/i });
    expect(within(list).getAllByRole('listitem')).toHaveLength(8);
  });
});
