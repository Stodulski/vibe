import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { CAT_OPEN, DETAIL } from '../../test/tournamentFixtures';
import { renderBooking } from '../../test/booking';
import { CategorySection } from '../../components/tournaments/CategorySection';
import { CancelledResult } from '../../components/registration/CancelRegistration';
import { CancelledEventNotice, FullEventNotice, SignUpClosedNotice } from './EventPageStates';

const icon = (container: HTMLElement) => container.querySelector('svg')?.getAttribute('class') ?? '';

describe('status notices', () => {
  it('say a full event with an info icon, never a success check', () => {
    const { container } = render(<FullEventNotice />);
    expect(icon(container)).toContain('lucide-info');
  });

  it('say a closed sign-up with a lock', () => {
    const { container } = render(<SignUpClosedNotice />);
    expect(icon(container)).toContain('lucide-lock');
  });

  it('say a cancelled category with a ban, a closed one with a lock and a full one with info', () => {
    const cases = [
      { status: 'cancelled', spots_left: 5, icon: 'lucide-ban' },
      { status: 'closed', spots_left: 5, icon: 'lucide-lock' },
      { status: 'open', spots_left: 0, icon: 'lucide-info' },
    ] as const;
    for (const c of cases) {
      const { container, unmount } = renderBooking(
        <CategorySection
          slug="test-club"
          detail={DETAIL}
          category={{ ...CAT_OPEN, status: c.status, spots_left: c.spots_left }}
          openFormId={null}
          onOpenFormChange={() => undefined}
        />,
      );
      expect(container.querySelector('[role="status"] svg')?.getAttribute('class')).toContain(c.icon);
      unmount();
    }
  });
});

describe('cancelled states', () => {
  it('shows the cancelled event with the same status icon size as the other states', () => {
    const { container } = render(<CancelledEventNotice />);
    expect(container.querySelector('h2')?.className).toContain('text-2xl');
  });

  it('shows the refund once, in the server sentence, not again as an amount', () => {
    const { container } = render(
      <CancelledResult
        refund={{ status: 'issued', message: 'Te devolvimos $11.000.', amount: 1_100_000 }}
        slug="test-club"
        title="Lugar cancelado"
      />,
    );
    expect(screen.getByText('Te devolvimos $11.000.')).toBeInTheDocument();
    expect(container.textContent.match(/\$11\.000/g)).toHaveLength(1);
    expect(icon(container)).toContain('lucide-ban');
  });
});
