import { describe, expect, it, vi } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import type { AvailabilitySlot, CourtAvailability } from '../../api/types';
import { BOOKING_MESSAGES } from '../../messages';
import { renderBooking } from '../../test/booking';
import { server } from '../../test/msw/server';
import { buildRulerHours } from './rulerHours';
import { TimeRuler } from './TimeRuler';

const t = BOOKING_MESSAGES.publicBooking;
const w = BOOKING_MESSAGES.waitlist;

function slot(start: string, available: boolean): AvailabilitySlot {
  const [h = '0', m = '0'] = start.split(':');
  return { start_time: start, end_time: '', start_min: Number(h) * 60 + Number(m), duration_minutes: 90, price: 1000, available };
}

function court(id: string, slots: AvailabilitySlot[]): CourtAvailability {
  return { court_id: id, court_name: `Pádel ${id}`, sport: 'padel', court_type: 'outdoor', has_lighting: true, slots };
}

const now = new Date('2099-12-05T09:00:00-03:00');

describe('TimeRuler — Avisarme cuando se libere', () => {
  it('asks to be told when any court frees at the taken hour, already chosen on the ruler', async () => {
    const user = userEvent.setup();
    let body: unknown = null;
    server.use(
      http.post('*/public/complexes/:slug/waitlist/places', async ({ request }) => {
        body = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const hours = buildRulerHours(
      [court('1', [slot('10:00', false), slot('11:00', true)]), court('2', [slot('10:00', false), slot('11:00', true)])],
      '2099-12-05',
      now,
    );
    renderBooking(
      <TimeRuler
        hours={hours}
        initialStartTime="10:00"
        severalCourts
        onSelectTime={vi.fn()}
        waitlist={{ slug: 'club', date: '2099-12-05' }}
      />,
    );

    expect(screen.getByRole('button', { name: `10:00, ${t.rulerTaken}` })).toHaveAttribute('aria-pressed', 'true');
    await user.click(screen.getByRole('button', { name: w.sendSlot }));

    const dialog = await screen.findByRole('alertdialog');
    expect(within(dialog).getByText(`${w.slotAnyCourtPrefix} 10:00.`)).toBeInTheDocument();
    expect(within(dialog).queryByRole('combobox')).not.toBeInTheDocument();
    await user.type(within(dialog).getByLabelText(w.emailLabel), 'ana@example.com');
    await user.click(within(dialog).getByRole('button', { name: w.sendSlot }));

    expect(await within(dialog).findByText(w.done)).toBeInTheDocument();
    expect(body).toEqual({
      kind: 'slot',
      court_ids: ['1', '2'],
      date: '2099-12-05',
      start_time: '10:00',
      duration_minutes: 90,
      email: 'ana@example.com',
    });
  });

  it('without a waitlist to join, a taken hour still points to the nearest free one', () => {
    const hours = buildRulerHours([court('1', [slot('10:00', false), slot('11:00', true)])], '2099-12-05', now);
    renderBooking(<TimeRuler hours={hours} initialStartTime="10:00" severalCourts={false} onSelectTime={vi.fn()} />);

    expect(screen.getByRole('button', { name: t.goToNearestFree })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: w.sendSlot })).not.toBeInTheDocument();
  });
});
