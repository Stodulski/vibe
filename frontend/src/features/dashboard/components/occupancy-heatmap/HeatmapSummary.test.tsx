import { render, screen, fireEvent } from '@testing-library/react';
import { HeatmapSummary } from './HeatmapSummary';

const baseProps = {
  isLoading: false,
  avgOccupancy: 42,
  peakDay: 'Lunes',
  peakHour: '20:00',
  expanded: false,
  onToggle: vi.fn(),
};

describe('HeatmapSummary', () => {
  it('calls onToggle when the expand button is pressed', () => {
    const onToggle = vi.fn();
    render(<HeatmapSummary {...baseProps} onToggle={onToggle} />);
    fireEvent.click(screen.getByRole('button', { name: /ver mapa/i }));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it('marks the toggle button collapsed/expanded via aria-expanded', () => {
    const { rerender } = render(<HeatmapSummary {...baseProps} expanded={false} />);
    expect(screen.getByRole('button', { name: /ver mapa/i })).toHaveAttribute('aria-expanded', 'false');

    rerender(<HeatmapSummary {...baseProps} expanded={true} />);
    expect(screen.getByRole('button', { name: /ocultar/i })).toHaveAttribute('aria-expanded', 'true');
  });

  it('points aria-controls at the heatmap panel it expands', () => {
    render(<HeatmapSummary {...baseProps} />);
    expect(screen.getByRole('button', { name: /ver mapa/i })).toHaveAttribute(
      'aria-controls',
      'occupancy-heatmap-panel',
    );
  });

  // The label is `hidden sm:inline` — an icon-only toggle on mobile with no
  // accessible name, before this fix. jsdom doesn't apply the stylesheet
  // that would hide it there, so this checks the fix's actual shape: an
  // always-present `.sr-only` twin.
  it('keeps an always-visible sr-only label on the mobile toggle', () => {
    render(<HeatmapSummary {...baseProps} />);
    const button = screen.getByRole('button', { name: /ver mapa/i });
    expect(button.querySelector('.sr-only')).not.toBeNull();
  });
});
