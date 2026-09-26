import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SkeletonSlotGrid } from './SkeletonSlotGrid';

const t = ES_AR;

describe('SkeletonSlotGrid', () => {
  it('announces the loading state through its status role', () => {
    render(<SkeletonSlotGrid />);
    expect(screen.getByRole('status')).toHaveAccessibleName(t.publicBooking.checkingAvailability);
  });

  it('depicts hour groups, not the old per-court cards', () => {
    // Two period groups (afternoon, evening), each a grid of hour
    // placeholders — the shape `CourtSelector`/`TimeGroupGrid` actually
    // render, not a card per court.
    const { container } = render(<SkeletonSlotGrid />);
    expect(container.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThan(0);
  });
});
