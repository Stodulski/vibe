import { describe, it, expect, vi } from 'vitest';
import { renderWithProviders, screen, userEvent } from '@/test/test-utils';
import { CreateBookingModal } from './CreateBookingModal';
import { ES_AR } from '@/shared/i18n/es_AR';

vi.mock('../hooks/useBookings', () => ({ useBookings: () => ({ data: [], isLoading: false }) }));

const t = ES_AR.bookings.series;

function renderModal() {
  renderWithProviders(
    <CreateBookingModal open onClose={vi.fn()} complexId="c1" courts={[]} depositPercentage={0} schedules={[]} />,
  );
}

describe('CreateBookingModal — repeat every week', () => {
  it('shows the switch off, with no end choice until it is turned on', () => {
    renderModal();

    expect(screen.getByRole('switch', { name: t.repeatWeekly })).not.toBeChecked();
    expect(screen.queryByText(t.endLabel)).not.toBeInTheDocument();
  });

  it('turning it on reveals the end choice, defaulting to "Sin fecha de fin"', async () => {
    const user = userEvent.setup();
    renderModal();

    await user.click(screen.getByRole('switch', { name: t.repeatWeekly }));

    expect(screen.getByText(t.endLabel)).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: t.endLabel })).toHaveTextContent(t.endNever);
    expect(screen.queryByLabelText(t.weeksLabel)).not.toBeInTheDocument();
  });
});
