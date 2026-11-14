import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SeriesConflictsPanel } from './SeriesConflictsPanel';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CourtAlternative } from '@/shared/types/api.types';

const t = ES_AR.bookings.series;

function renderPanel(endless = false, alternatives: CourtAlternative[] = []) {
  const onSkip = vi.fn();
  const onBack = vi.fn();
  const onUseCourt = vi.fn();
  render(
    <SeriesConflictsPanel
      conflicts={[
        { date: '2026-11-18', reason: 'overlap', held_by: 'event' },
        { date: '2026-11-25', reason: 'closed_day' },
        { date: '2026-12-02', reason: 'no_price' },
      ]}
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

describe('SeriesConflictsPanel', () => {
  it('lists every refused date with its reason', () => {
    renderPanel();

    expect(screen.getByText(t.conflictsTitle(3))).toBeInTheDocument();
    const items = screen.getAllByRole('listitem').map((li) => li.textContent);
    expect(items).toEqual([
      'mié 18 nov 2026: ocupada por un evento',
      'mié 25 nov 2026: el complejo está cerrado',
      'mié 2 dic 2026: sin precio para ese horario',
    ]);
  });

  it('names what holds each date: booking, fixed booking, block, event or tournament match', () => {
    expect(t.heldBy).toEqual({
      booking: 'ocupada por una reserva',
      fixed: 'ocupada por otro turno fijo',
      event: 'ocupada por un evento',
      match: 'ocupada por un partido de torneo',
    });
    expect(t.conflictReasons.blocked).toBe('ocupada por un bloqueo');
  });

  it('says how many dates collide in total, and that a series with no end was checked for the next year only', () => {
    renderPanel(true);

    expect(screen.getByText(t.conflictsTitle(3))).toBeInTheDocument();
    expect(screen.getByText(t.conflictsEndless)).toBeInTheDocument();
  });

  it('does not talk about the next year for a series that ends', () => {
    renderPanel(false);

    expect(screen.queryByText(t.conflictsEndless)).not.toBeInTheDocument();
  });

  it('offers to create the series skipping those dates, or to go back', async () => {
    const user = userEvent.setup();
    const { onSkip, onBack } = renderPanel();

    await user.click(screen.getByRole('button', { name: t.skipConflicts }));
    await user.click(screen.getByRole('button', { name: t.conflictsBack }));

    expect(onSkip).toHaveBeenCalledOnce();
    expect(onBack).toHaveBeenCalledOnce();
  });
});

describe('SeriesConflictsPanel, other courts', () => {
  it('offers the free courts, and switches to the one the owner picks', async () => {
    const user = userEvent.setup();
    const { onUseCourt } = renderPanel(false, [
      { court_id: 'ct2', court_name: 'Cancha 2', conflicts: 0 },
      { court_id: 'ct3', court_name: 'Cancha 3', conflicts: 2 },
    ]);

    expect(screen.getByText(t.alternativesFree)).toBeInTheDocument();
    const rows = screen.getAllByTestId('court-alternative').map((row) => row.textContent.replace(/\u00A0/g, ' '));
    expect(rows).toEqual([
      'Cancha 2 (todas las semanas libres) · Usar esta cancha',
      'Cancha 3 (2 fechas ocupadas) · Usar esta cancha',
    ]);
    const [, second] = screen.getAllByRole('button', { name: t.useCourt });
    if (second) await user.click(second);

    expect(onUseCourt).toHaveBeenCalledWith('ct3');
  });

  it('says "menos fechas ocupadas" when no other court is free every week', () => {
    renderPanel(false, [{ court_id: 'ct3', court_name: 'Cancha 3', conflicts: 1 }]);

    expect(screen.getByText(t.alternativesFewer)).toBeInTheDocument();
    expect(screen.queryByText(t.alternativesFree)).not.toBeInTheDocument();
  });

  it('shows nothing about other courts when the server offers none', () => {
    renderPanel();

    expect(screen.queryByTestId('court-alternative')).not.toBeInTheDocument();
  });
});
