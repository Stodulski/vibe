import { test, expect, type Page } from '@playwright/test';
import { ES_AR } from '../../src/shared/i18n/es_AR';
import { BOOKING_MESSAGES } from '../../../packages/booking/src/messages';
import { landingComplexUrl, waitForStorefrontHydration } from '../helpers/landing';
import { futureWeekday, openOwnerPage, provisionWorld, type EventsWorld } from '../helpers/events-fixtures';
import { payAtCheckout, paymentOfPreference, waitForCheckout, waitForRefundOf } from '../helpers/mp-checkout';
import type { FakePayment } from '../mp-fake/client';

/**
 * A canchas libres sign-up paid through the fake Mercado Pago (e2e/mp-fake),
 * and the refund the event's cancellation sends back (odd/tasks/mp-fake-e2e.md
 * S1d). `open-play-events.spec.ts` covers the event itself, with its paid
 * participants seeded as setup.
 *
 * Its own owner and complex, one court. Tests run in order and share the event.
 */
test.describe.configure({ mode: 'serial' });

const te = ES_AR.events;
const pb = BOOKING_MESSAGES.publicBooking;
const EVENT_NAME = 'Americano pago E2E';
const PRICE_PESOS = 4000;
// A Thursday about seven weeks out.
const DATE = futureWeekday(4, 48);
const PERSON = { firstName: 'Paga', lastName: 'Libre', phone: '1155550301' };

let world: EventsWorld;
let owner: { page: Page; close: () => Promise<void> };
let eventId = '';
let payment: FakePayment;

test.beforeAll(async ({ browser }) => {
  test.setTimeout(120_000);
  world = await provisionWorld('open-play-mp', 1);
  const court = world.courts[0];
  if (!court) throw new Error('no court');
  const created = await world.post<{ event: { id: string } }>(`/complexes/${world.complexId}/open-play-events`, {
    name: EVENT_NAME,
    sport: 'padel',
    date: DATE,
    start_time: '20:00',
    end_time: '22:00',
    court_ids: [court.id],
    capacity: 4,
    price: PRICE_PESOS * 100,
  });
  eventId = created.event.id;
  const opened = await openOwnerPage(browser, world.owner);
  owner = { page: opened.page, close: () => opened.context.close() };
});

test.afterAll(async () => {
  await owner.close();
});

test('a sign-up paid at checkout confirms the spot, and the owner sees it paid', async ({ page }) => {
  test.slow();
  await page.goto(landingComplexUrl(world.slug, `/events/${eventId}`));
  await waitForStorefrontHydration(page);
  await expect(page.getByRole('heading', { level: 1, name: EVENT_NAME })).toBeVisible({ timeout: 15_000 });
  await page.locator('#event_first_name').fill(PERSON.firstName);
  await page.locator('#event_last_name').fill(PERSON.lastName);
  await page.locator('#event_phone').fill(PERSON.phone);
  await page.getByRole('button', { name: new RegExp(pb.paySecurely) }).click();

  const preferenceId = await waitForCheckout(page);
  await payAtCheckout(page, 'approve');

  // back_urls.success: the event page, carrying the sign-up's token.
  await expect(page).toHaveURL(new RegExp(`/events/${eventId}\\?token=.*registered=1`), { timeout: 15_000 });
  await expect(page.getByText(BOOKING_MESSAGES.events.confirmedTitle)).toBeVisible({ timeout: 30_000 });
  payment = await paymentOfPreference(preferenceId);
  expect(payment.status).toBe('approved');

  await owner.page.goto(`/events/open-play?event=${eventId}`);
  const sheet = owner.page.getByRole('dialog');
  await expect(sheet.getByText(`${te.detail.participants} · ${te.detail.confirmedCount(1)}`)).toBeVisible({
    timeout: 10_000,
  });
  const row = sheet
    .getByRole('list', { name: te.detail.participants })
    .getByRole('listitem')
    .filter({ hasText: `${PERSON.firstName} ${PERSON.lastName}` });
  await expect(row).toContainText(te.participant.paid(`$${PRICE_PESOS.toLocaleString('es-AR')}`));
});

test('cancelling the event refunds the paid sign-up on Mercado Pago', async () => {
  const sheet = owner.page.getByRole('dialog');
  await sheet.getByRole('button', { name: te.cancel.action }).click();
  const confirm = owner.page.getByRole('alertdialog');
  await expect(confirm.getByText(te.cancel.confirmDescriptionPaid(1))).toBeVisible();
  await confirm.getByRole('button', { name: te.cancel.confirmLabel }).click();
  await expect(owner.page.getByText(new RegExp(`^${te.cancel.success(0)}`))).toBeVisible({ timeout: 10_000 });

  // The whole payment goes back, service fee included.
  const refunds = await waitForRefundOf(payment.id, payment.transaction_amount);
  expect(refunds.every((r) => r.status === 'approved')).toBe(true);
});
