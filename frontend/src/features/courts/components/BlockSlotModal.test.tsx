import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { BlockSlotModal } from './BlockSlotModal';
import type { CourtWithPrices } from '@/shared/types/api.types';

const t = ES_AR;

const mockMutate = vi.fn();

vi.mock('../hooks/useBlockCourtSlot', () => ({
  useBlockCourtSlot: () => ({ mutate: mockMutate, isPending: false }),
}));

vi.mock('../hooks/useBlockedSlots', () => ({
  useBlockedSlots: () => ({ data: [] }),
}));

vi.mock('@/shared/hooks/useBookingsByDate', () => ({
  useBookingsByDate: () => ({ data: [] }),
}));

const courts: CourtWithPrices[] = [
  {
    id: 'ct1',
    complex_id: 'c1',
    name: 'Cancha 1',
    sport: 'padel',
    court_type: 'outdoor',
    is_active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices: [],
  },
];

describe('BlockSlotModal', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // Every control used to render from a bare `<Label>` with no `htmlFor`, so
  // its accessible name came from whatever placeholder text happened to be
  // showing — nothing a screen reader could announce on focus.
  it('associates a label with each of the four controls', () => {
    renderWithProviders(<BlockSlotModal open onClose={vi.fn()} complexId="c1" courts={courts} />);

    expect(screen.getByRole('button', { name: t.courts.blockDate })).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: t.bookings.court })).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: t.courts.blockStartTime })).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: t.courts.blockEndTime })).toBeInTheDocument();
    expect(screen.getByLabelText(t.courts.blockReason)).toBeInTheDocument();
  });

  // The old `useState(prefill?.date ?? '')` copied `prefill` into state once,
  // at first mount, and never again — a parent that changes `prefill` while
  // the modal stays open had that change silently ignored until the next
  // close/reopen. Keying the form body by `prefill` (see `BlockSlotModal`)
  // remounts it with the new prefill instead.
  it('picks up a new prefill while the modal stays open', () => {
    const { rerender } = renderWithProviders(<BlockSlotModal open onClose={vi.fn()} complexId="c1" courts={courts} />);

    expect(screen.getByRole('button', { name: t.courts.blockDate })).toHaveTextContent(t.bookings.selectDate);

    rerender(
      <BlockSlotModal
        open
        onClose={vi.fn()}
        complexId="c1"
        courts={courts}
        prefill={{ court_id: 'ct1', date: '2026-06-15', start_time: '10:00' }}
      />,
    );

    expect(screen.getByRole('button', { name: t.courts.blockDate })).not.toHaveTextContent(t.bookings.selectDate);
    expect(screen.getByRole('combobox', { name: t.bookings.court })).toHaveTextContent('Cancha 1');
  });
});
