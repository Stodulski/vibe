import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TooltipProvider } from '@/shared/components/ui/tooltip';
import { DesktopHeatmap } from './DesktopHeatmap';
import { heatKey } from './heatmapUtils';

function renderHeatmap(dataMap = new Map<string, number>()) {
  return render(
    <TooltipProvider>
      <DesktopHeatmap dataMap={dataMap} />
    </TooltipProvider>,
  );
}

describe('DesktopHeatmap', () => {
  it('renders every cell with an aria-label carrying its day, hour and value', () => {
    const dataMap = new Map([[heatKey(1, 10), 42]]);
    renderHeatmap(dataMap);
    expect(screen.getByLabelText(/Lunes 10:00: 42%/)).toBeInTheDocument();
  });

  // Before this fix, the tooltip trigger was a plain, non-focusable `div` —
  // its occupancy value was only reachable by hovering with a mouse.
  it('puts every cell in the tab order so its tooltip is reachable by keyboard', () => {
    renderHeatmap();
    const cells = screen.getAllByRole('cell');
    expect(cells.length).toBeGreaterThan(0);
    for (const cell of cells) {
      expect(cell).toHaveAttribute('tabindex', '0');
    }
  });
});
