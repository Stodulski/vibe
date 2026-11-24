import { describe, it, expect, vi } from 'vitest';
import { http, HttpResponse } from 'msw';
import { renderWithProviders, screen } from '@/test/test-utils';
import { server } from '@/test/msw/server';
import { makeCourt, makePrice } from '@/test/factories';
import type { CourtPrice, Schedule } from '@/shared/types/api.types';
import { ScheduleConfig } from './ScheduleConfig';

const mockUseSchedules = vi.fn();
vi.mock('../hooks/useSchedules', () => ({
  useSchedules: (...args: unknown[]) => mockUseSchedules(...args) as unknown,
}));
vi.mock('../hooks/useUpdateSchedules', () => ({
  useUpdateSchedules: () => ({ mutate: vi.fn(), isPending: false }),
}));

function day(d: Schedule['day'], openTime: string, closeTime: string): Schedule {
  return { id: d, complex_id: 'c1', day: d, open_time: openTime, close_time: closeTime, is_closed: false };
}

/** Courts come from the real query (MSW), the same one the courts page reads. */
function renderWith(schedules: Schedule[], prices: CourtPrice[]) {
  mockUseSchedules.mockReturnValue({ data: schedules, isLoading: false, isError: false, refetch: vi.fn() });
  server.use(
    http.get('*/complexes/:complexId/courts', () =>
      HttpResponse.json({ courts: [{ ...makeCourt({ name: 'Cancha 1' }), prices }] }),
    ),
  );
  renderWithProviders(<ScheduleConfig complexId="c1" slug="club" />);
}

describe('ScheduleConfig — opening hours no price covers', () => {
  // Opening until 01:00 on a day priced only up to 23:00: those two hours are
  // open but have no price, and an unpriced slot is never offered.
  it('names the hours outside every price band and links to the prices', async () => {
    renderWith(
      [day('monday', '08:00', '01:00')],
      [makePrice({ day_type: 'monday', time_from: '08:00', time_to: '23:00' })],
    );

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Cancha 1');
    expect(alert).toHaveTextContent('Lunes 23:00 a 01:00');
    expect(screen.getByRole('link', { name: 'Ir a precios' })).toHaveAttribute('href', '/courts');
  });

  // A day the court has no price for at all is the owner's choice ("this day
  // has no rate", as the price dialog puts it), not a gap the new hours opened.
  it('says nothing when the priced days are covered', async () => {
    renderWith(
      [day('monday', '08:00', '23:00'), day('tuesday', '08:00', '01:00')],
      [makePrice({ day_type: 'monday', time_from: '08:00', time_to: '23:00' })],
    );

    await screen.findByRole('button', { name: 'Guardar' });
    // The courts query resolves after the form: give it the same chance.
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
