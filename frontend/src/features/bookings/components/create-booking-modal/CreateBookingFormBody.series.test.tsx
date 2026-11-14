import { describe, it, expect, vi } from 'vitest';
import { useEffect } from 'react';
import { renderWithProviders, screen } from '@/test/test-utils';
import { makeCourt } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CreateBookingFormBody } from './CreateBookingFormBody';
import { useCreateBookingForm } from './useCreateBookingForm';

vi.mock('../../api/bookingSeries.api', () => ({ bookingSeriesApi: { create: vi.fn() } }));
vi.mock('../../api/bookings.api', () => ({ bookingsApi: { create: vi.fn(), list: vi.fn() } }));

const t = ES_AR.bookings;

// 2099-01-05 is a Monday.
function LastStep({ repeat }: { repeat: boolean }) {
  const form = useCreateBookingForm({
    open: false,
    complexId: 'c1',
    courts: [{ ...makeCourt({ id: 'ct1', name: 'Cancha 1' }), prices: [] }],
    depositPercentage: 0,
    schedules: [],
  });
  const { setValue } = form;
  const { setRepeat } = form.series;
  useEffect(() => {
    setValue('court_id', 'ct1');
    setValue('date', '2099-01-05');
    setValue('start_time', '18:00');
    setValue('duration_minutes', 90);
    setRepeat(repeat);
  }, [setValue, setRepeat, repeat]);
  return <CreateBookingFormBody step={3} form={form} onClose={vi.fn()} onNext={vi.fn()} onBack={vi.fn()} />;
}

describe('CreateBookingFormBody — last step', () => {
  it('a weekly booking says it repeats and is created as a turno fijo', async () => {
    renderWithProviders(<LastStep repeat />);

    expect(await screen.findByText('Todos los lunes')).toBeInTheDocument();
    expect(screen.getByText('18:00 a 19:30')).toBeInTheDocument();
    expect(screen.getByText('Sin fecha de fin')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: t.series.createLabel })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.create })).not.toBeInTheDocument();
  });

  it('an ordinary booking keeps "Nueva reserva" and no weekly summary', async () => {
    renderWithProviders(<LastStep repeat={false} />);

    expect(await screen.findByRole('button', { name: t.create })).toBeInTheDocument();
    expect(screen.queryByText(/Todos los lunes/)).not.toBeInTheDocument();
  });
});
