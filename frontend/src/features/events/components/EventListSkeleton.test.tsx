import { describe, it, expect } from 'vitest';
import { renderWithProviders as render, screen } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { EventListSkeleton } from './EventListSkeleton';

describe('EventListSkeleton', () => {
  it('is the one loading status of the list, with four rows hidden from assistive tech', () => {
    render(<EventListSkeleton />);

    const status = screen.getByRole('status', { name: ES_AR.common.loading });
    expect(status.children).toHaveLength(4);
    for (const row of Array.from(status.children)) {
      expect(row).toHaveAttribute('aria-hidden', 'true');
    }
  });
});
