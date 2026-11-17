import { describe, expect, it } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import type { CourtAvailability } from '../../api/types';
import { BOOKING_MESSAGES } from '../../messages';
import { renderBooking } from '../../test/booking';
import { chooseOption } from '../../test/chooseOption';
import { server } from '../../test/msw/server';
import { MissingSlotWaitlist } from './MissingSlotWaitlist';

const t = BOOKING_MESSAGES.waitlist;

function court(available: boolean): CourtAvailability {
  return {
    court_id: 'c1',
    court_name: 'Pádel 1',
    sport: 'padel',
    court_type: 'outdoor',
    has_lighting: true,
    slots: [
      { start_time: '19:00', end_time: '20:30', start_min: 1140, duration_minutes: 90, price: 1000, available },
    ],
  };
}

describe('MissingSlotWaitlist', () => {
  it('opens a popup with the taken slots and an email, and saves the chosen slot', async () => {
    const user = userEvent.setup();
    let body: unknown = null;
    server.use(
      http.post('*/public/complexes/:slug/waitlist/places', async ({ request }) => {
        body = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );
    renderBooking(<MissingSlotWaitlist slug="club" date="2099-12-05" courts={[court(false)]} />);

    await user.click(screen.getByRole('button', { name: t.missingSlot }));
    const dialog = await screen.findByRole('alertdialog');
    await chooseOption(user, within(dialog).getByRole('combobox', { name: t.slotLabel }), '19:00 · Pádel 1');
    await user.type(within(dialog).getByLabelText(t.emailLabel), 'ana@example.com');
    await user.click(within(dialog).getByRole('button', { name: t.sendSlot }));

    expect(await within(dialog).findByText(t.done)).toBeInTheDocument();
    expect(body).toEqual({
      kind: 'slot',
      court_id: 'c1',
      date: '2099-12-05',
      start_time: '19:00',
      duration_minutes: 90,
      email: 'ana@example.com',
    });
  });

  it('asks for a slot before sending', async () => {
    const user = userEvent.setup();
    renderBooking(<MissingSlotWaitlist slug="club" date="2099-12-05" courts={[court(false)]} />);

    await user.click(screen.getByRole('button', { name: t.missingSlot }));
    const dialog = await screen.findByRole('alertdialog');
    await user.type(within(dialog).getByLabelText(t.emailLabel), 'ana@example.com');
    await user.click(within(dialog).getByRole('button', { name: t.sendSlot }));

    expect(within(dialog).getByText(t.slotRequired)).toBeInTheDocument();
  });

  it('is not offered when no slot of the day is taken', () => {
    renderBooking(<MissingSlotWaitlist slug="club" date="2099-12-05" courts={[court(true)]} />);

    expect(screen.queryByRole('button', { name: t.missingSlot })).not.toBeInTheDocument();
  });
});
