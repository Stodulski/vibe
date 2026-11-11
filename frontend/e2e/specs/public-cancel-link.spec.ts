import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { bookFirstFreeHour } from '../helpers/booking-fixtures';
import { createApiHelper } from '../helpers/api.helper';
import type { ApiBooking } from '../helpers/api.helper';
import { getFutureDate } from '../helpers/test-data';
import { landingComplexUrl, waitForStorefrontHydration } from '../helpers/landing';
import { mintBookingLinkToken } from '../helpers/e2e-db';

/**
 * The client's own cancellation link, on the storefront, with a real token.
 * `public-cancel.spec.ts` only covers the link with no token at all.
 *
 * The booking is paid in cash at the counter and sits well outside the
 * cancellation window, so the client may cancel and is owed the money back by
 * hand: the page must say the complex returns it (never promise an automatic
 * refund), and the booking ends cancelled with refund_status 'partial', the
 * state that offers the owner "Marcar devolución realizada".
 */
test.describe('Public cancellation link', () => {
  test('a client cancels a cash-paid booking from their link and is told the complex returns the money', async ({
    page,
  }) => {
    const { complexId, complexSlug, courtId } = await getSharedSetup();
    const api = await createApiHelper();
    const lastName = `Link${String(Date.now())}`;
    const booking = await bookFirstFreeHour(api, complexId, courtId, getFutureDate(5), {
      firstName: 'Cancela',
      lastName,
    });
    await api.confirmPayment(complexId, booking.id, booking.price);
    const token = mintBookingLinkToken(booking.id, complexId);
    const cancelUrl = landingComplexUrl(complexSlug, `/book/cancel?token=${token}`);

    await page.goto(cancelUrl);
    await waitForStorefrontHydration(page);
    const main = page.getByRole('main');
    await expect(main.getByRole('heading', { name: '¿Cancelás la reserva?' })).toBeVisible({ timeout: 10_000 });
    await expect(main.getByText('El complejo te devuelve lo que pagaste. Coordinalo con ellos.')).toBeVisible();

    await main.getByRole('button', { name: 'Confirmar cancelación' }).click();
    const confirm = page.getByRole('alertdialog').or(page.getByRole('dialog'));
    await expect(confirm.getByText('¿Cancelar la reserva?')).toBeVisible({ timeout: 5_000 });
    await expect(confirm.getByText('El complejo te devuelve en mano lo que pagaste.')).toBeVisible();

    const response = page.waitForResponse(
      (res) => res.url().endsWith('/book/cancel') && res.request().method() === 'POST',
    );
    await confirm.getByRole('button', { name: 'Sí, cancelar' }).click();
    expect((await response).ok()).toBe(true);
    await expect(main.getByText('Reserva cancelada')).toBeVisible({ timeout: 10_000 });

    const after = await api.get<{ booking: ApiBooking }>(`/complexes/${complexId}/bookings/${booking.id}`);
    expect(after.booking.status).toBe('cancelled');
    expect(after.booking.refund_status).toBe('partial');

    // Opening the same link again cannot cancel twice.
    await page.goto(cancelUrl);
    await waitForStorefrontHydration(page);
    await expect(main.getByText('Esta reserva ya fue cancelada')).toBeVisible({ timeout: 10_000 });
    await expect(main.getByRole('button', { name: 'Confirmar cancelación' })).toHaveCount(0);
  });
});
