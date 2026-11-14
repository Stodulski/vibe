import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SeriesConflictsPanel } from './SeriesConflictsPanel';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR.bookings.series;

function renderPanel() {
  const onSkip = vi.fn();
  const onBack = vi.fn();
  render(
    <SeriesConflictsPanel
      conflicts={[
        { date: '2026-11-18', reason: 'overlap' },
        { date: '2026-11-25', reason: 'closed_day' },
        { date: '2026-12-02', reason: 'no_price' },
      ]}
      pending={false}
      onSkip={onSkip}
      onBack={onBack}
    />,
  );
  return { onSkip, onBack };
}

describe('SeriesConflictsPanel', () => {
  it('lists every refused date with its reason', () => {
    renderPanel();

    expect(screen.getByText(t.conflictsTitle)).toBeInTheDocument();
    const items = screen.getAllByRole('listitem').map((li) => li.textContent);
    expect(items).toEqual([
      'mié 18 nov — ocupada',
      'mié 25 nov — el complejo está cerrado',
      'mié 2 dic — sin precio para ese horario',
    ]);
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
