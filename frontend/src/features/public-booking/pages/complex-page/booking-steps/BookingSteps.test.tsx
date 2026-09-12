import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BookingSteps } from './BookingSteps';

function renderSteps(overrides: Partial<Parameters<typeof BookingSteps>[0]> = {}) {
  const onDurationChange = vi.fn();
  const onEditTime = vi.fn();
  const onSportFilterChange = vi.fn();
  const utils = render(
    <BookingSteps
      availableSports={['padel']}
      sportFilter={null}
      onSportFilterChange={onSportFilterChange}
      duration={60}
      selectedDuration={60}
      onDurationChange={onDurationChange}
      timeAnswer="08:30"
      onEditTime={onEditTime}
      answeredFromUrl={{ sport: false, duration: true }}
      {...overrides}
    >
      <div>HOURS</div>
    </BookingSteps>,
  );
  return { ...utils, onDurationChange, onEditTime, onSportFilterChange };
}

describe('BookingSteps — answered questions collapse to crumbs', () => {
  it('shows the duration and time crumbs plus the hours when both are already answered', () => {
    renderSteps();

    expect(screen.getByRole('button', { name: /Duración/ })).toHaveTextContent('60 min');
    expect(screen.getByRole('button', { name: /Horario/ })).toHaveTextContent('08:30');
    expect(screen.getByText('HOURS')).toBeInTheDocument();
  });

  it('reopens the duration question on its crumb, hiding the hours and the time crumb', async () => {
    const user = userEvent.setup();
    renderSteps();

    await user.click(screen.getByRole('button', { name: /Duración/ }));

    expect(screen.getByText('¿Cuánto tiempo?')).toBeInTheDocument();
    expect(screen.queryByText('HOURS')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Horario/ })).not.toBeInTheDocument();
  });

  it('reports the newly chosen duration once an answer is picked from the reopened question', async () => {
    const user = userEvent.setup();
    const { onDurationChange } = renderSteps();

    await user.click(screen.getByRole('button', { name: /Duración/ }));
    await user.click(screen.getByRole('button', { name: '90 min' }));

    expect(onDurationChange).toHaveBeenCalledWith(90);
  });

  // Re-opening an already-answered question is a comparison ("was it 90 or
  // 60?"), and a comparison needs the current answer marked — this is the
  // one case where a duration option SHOULD render as selected.
  it('marks the current duration as selected when its question is reopened from the crumb', async () => {
    const user = userEvent.setup();
    renderSteps();

    await user.click(screen.getByRole('button', { name: /Duración/ }));

    expect(screen.getByRole('button', { name: '60 min' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: '90 min' })).toHaveAttribute('aria-pressed', 'false');
  });
});

describe('BookingSteps — the time crumb returns to the hours', () => {
  it('calls onEditTime when the time crumb is clicked', async () => {
    const user = userEvent.setup();
    const { onEditTime } = renderSteps();

    await user.click(screen.getByRole('button', { name: /Horario/ }));

    expect(onEditTime).toHaveBeenCalledTimes(1);
  });
});

describe('BookingSteps — nothing answered yet', () => {
  it('renders only the open question, with no crumbs and no children', () => {
    renderSteps({ answeredFromUrl: { sport: false, duration: false }, selectedDuration: null, timeAnswer: null });

    expect(screen.getByText('¿Cuánto tiempo?')).toBeInTheDocument();
    expect(screen.queryByText('HOURS')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Horario/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Duración/ })).not.toBeInTheDocument();
  });

  // U-01: 90 min used to render pressed here — `duration` (the availability
  // fetch's default) was the only value the question had to work with, and
  // it painted the answer before the visitor gave one.
  it('marks no duration option as selected before the visitor picks one', () => {
    renderSteps({ answeredFromUrl: { sport: false, duration: false }, selectedDuration: null, timeAnswer: null });

    for (const option of ['60 min', '90 min', '120 min']) {
      expect(screen.getByRole('button', { name: option })).toHaveAttribute('aria-pressed', 'false');
    }
  });
});
