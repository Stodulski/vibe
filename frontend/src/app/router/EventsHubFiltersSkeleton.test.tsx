import { describe, it, expect } from 'vitest';
import { renderWithProviders as render } from '@/test/test-utils';
import { EventsHubFiltersSkeleton } from './EventsHubFiltersSkeleton';

describe('EventsHubFiltersSkeleton', () => {
  it('draws four fields, one for each filter, hidden from assistive tech', () => {
    const { container } = render(<EventsHubFiltersSkeleton />);

    const bar = container.firstElementChild;
    expect(bar).toHaveAttribute('aria-hidden', 'true');
    expect(bar?.children).toHaveLength(4);
  });
});
