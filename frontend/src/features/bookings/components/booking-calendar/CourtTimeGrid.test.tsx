import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { CourtTimeGrid } from './CourtTimeGrid';
import { GRID_HEIGHT_PX, MIN_COLUMN_WIDTH_PX } from './gridLayout';
import type { CourtWithPrices } from '@/shared/types/api.types';

/**
 * happy-dom never lays anything out, so the grid's plot area measures 0 and
 * every column would fall back to the minimum. The hook is the seam: mocking
 * it hands the grid a viewport width the way a real ResizeObserver would, and
 * what is under test is what the grid then does with that number.
 */
const mockWidth = vi.fn(() => 0);
vi.mock('@/shared/hooks/useElementWidth', () => ({
  useElementWidth: () => ({ ref: () => undefined, width: mockWidth() }),
}));

/**
 * Stretching is a phone-only rule (`useColumnWidth`). happy-dom's viewport is
 * a desktop's, so the breakpoint is mocked: every test below runs as a phone
 * unless it says otherwise.
 */
const mockStretch = vi.fn(() => true);
vi.mock('@/shared/hooks/useMediaQuery', () => ({
  useMediaQuery: () => mockStretch(),
}));

const PLOT_WIDTH_PX = 600;

function makeCourts(count: number): CourtWithPrices[] {
  return Array.from({ length: count }, (_, i) => ({
    id: `ct${String(i + 1)}`,
    complex_id: 'c1',
    name: `Cancha ${String(i + 1)}`,
    sport: 'padel' as const,
    court_type: 'outdoor' as const,
    is_active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices: [],
  }));
}

/** A date far enough ahead that the now-line never renders and every slot is free. */
const FUTURE_DATE = '2027-03-20';
const SLOTS = ['10:00', '10:30', '11:00'];

function renderGrid(courtCount: number) {
  render(
    <CourtTimeGrid
      bookings={[]}
      blockedSlots={[]}
      activeCourts={makeCourts(courtCount)}
      date={FUTURE_DATE}
      slots={SLOTS}
      onSelectBooking={vi.fn()}
      onCreateBooking={vi.fn()}
    />,
  );

  // The court name lives in the header cell; the cell around it is what must
  // line up with the column below.
  const headerCell = screen.getAllByText(/^Cancha \d$/)[0]?.parentElement;
  // Free-slot controls are direct children of their court's column, and only
  // that column carries a full-height inline box.
  const column = screen.getAllByRole('button')[0]?.closest('[style*="height: 100%"]');
  // The plot area is the columns' own parent — the box whose width has to
  // total them, so the gridlines span every column rather than the visible
  // slice of the scroller.
  const body = column?.parentElement;
  // Guards the lookup above: the plot area is the only box drawn at the grid's
  // full day height, so a wrong element here fails loudly instead of making
  // the width assertions vacuous.
  expect(body).toHaveStyle({ height: `${String(GRID_HEIGHT_PX)}px` });

  return { headerCell, column, body };
}

describe('CourtTimeGrid column width', () => {
  beforeEach(() => {
    mockWidth.mockReturnValue(PLOT_WIDTH_PX);
    mockStretch.mockReturnValue(true);
  });

  it('stretches a lone court across the whole plot area', () => {
    // The bug: one court on a phone was drawn as a 200px strip with the rest
    // of the viewport left empty.
    const { headerCell, column, body } = renderGrid(1);

    expect(column).toHaveStyle({ width: `${String(PLOT_WIDTH_PX)}px` });
    expect(headerCell).toHaveStyle({ width: `${String(PLOT_WIDTH_PX)}px` });
    expect(body).toHaveStyle({ width: `${String(PLOT_WIDTH_PX)}px` });
  });

  it('holds every column at the minimum once five courts no longer fit', () => {
    // 600 / 5 is below the floor, so nothing changes from the old behaviour:
    // the columns overflow and the grid scrolls sideways.
    const { headerCell, column, body } = renderGrid(5);

    expect(column).toHaveStyle({ width: `${String(MIN_COLUMN_WIDTH_PX)}px` });
    expect(headerCell).toHaveStyle({ width: `${String(MIN_COLUMN_WIDTH_PX)}px` });
    expect(body).toHaveStyle({ width: `${String(5 * MIN_COLUMN_WIDTH_PX)}px` });
  });

  it('draws the header cell and the column it labels at the very same width', () => {
    // Two courts in 600 stretch to 300 each — a width neither the minimum nor
    // the plot area, so a component still reading the old constant would show.
    const { headerCell, column, body } = renderGrid(2);

    expect(column).toHaveStyle({ width: '300px' });
    expect(headerCell).toHaveStyle({ width: '300px' });
    expect(body).toHaveStyle({ width: `${String(PLOT_WIDTH_PX)}px` });
  });

  it('keeps one fixed width per court from the sm breakpoint up, however wide the plot is', () => {
    mockStretch.mockReturnValue(false);
    const { column, headerCell, body } = renderGrid(1);

    expect(column).toHaveStyle({ width: `${String(MIN_COLUMN_WIDTH_PX)}px` });
    expect(headerCell).toHaveStyle({ width: `${String(MIN_COLUMN_WIDTH_PX)}px` });
    expect(body).toHaveStyle({ width: `${String(MIN_COLUMN_WIDTH_PX)}px` });
  });

  it('falls back to the minimum while the plot area is still unmeasured', () => {
    // Width is 0 on the first render and wherever ResizeObserver is missing —
    // server rendering included. Columns must not collapse there.
    mockWidth.mockReturnValue(0);
    const { headerCell, column } = renderGrid(1);

    expect(column).toHaveStyle({ width: `${String(MIN_COLUMN_WIDTH_PX)}px` });
    expect(headerCell).toHaveStyle({ width: `${String(MIN_COLUMN_WIDTH_PX)}px` });
  });
});
