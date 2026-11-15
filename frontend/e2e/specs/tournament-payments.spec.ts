import { test, expect, type Page } from '@playwright/test';
import { ES_AR } from '../../src/shared/i18n/es_AR';
import { BOOKING_MESSAGES } from '../../../packages/booking/src/messages';
import { landingComplexUrl, waitForStorefrontHydration } from '../helpers/landing';
import { addDays, futureWeekday, openOwnerPage, provisionWorld, type EventsWorld } from '../helpers/events-fixtures';
import { payAtCheckout, paymentOfPreference, waitForCheckout, waitForRefundOf } from '../helpers/mp-checkout';
import type { FakePayment } from '../mp-fake/client';

/**
 * A tournament pair's sign-up paid through the fake Mercado Pago
 * (e2e/mp-fake), and the refund the category's cancellation sends back
 * (odd/tasks/mp-fake-e2e.md S1d). `tournaments.spec.ts` covers the tournament
 * itself, with its paid pairs seeded as setup.
 *
 * Its own owner and complex. Tests run in order and share the tournament.
 */
test.describe.configure({ mode: 'serial' });

const tt = ES_AR.tournaments;
const pb = BOOKING_MESSAGES.publicBooking;
const TOURNAMENT = 'Copa pagada E2E';
const CATEGORY = '7ma';
const ENTRY_PESOS = 7000;
// A weekend about eight weeks out.
const DAY1 = futureWeekday(6, 54);
const PAYER = { firstName: 'Ana', lastName: 'Pagadora', phone: '1155550401' };
const PARTNER = { firstName: 'Beto', lastName: 'Companero', phone: '1155550402' };

let world: EventsWorld;
let owner: { page: Page; close: () => Promise<void> };
let tournamentId = '';
let categoryId = '';
let payment: FakePayment;

test.beforeAll(async ({ browser }) => {
  test.setTimeout(120_000);
  world = await provisionWorld('tournaments-mp', 1);
  await world.post(`/complexes/${world.complexId}/tournaments`, {
    name: TOURNAMENT,
    sport: 'padel',
    start_date: DAY1,
    end_date: addDays(DAY1, 1),
    categories: [{ name: CATEGORY, format: 'elimination', capacity: 8, price: ENTRY_PESOS * 100 }],
  });
  const { tournaments } = await world.api.get<{
    tournaments: { id: string; name: string; categories: { id: string; name: string }[] }[];
  }>(`/complexes/${world.complexId}/tournaments?date_from=${DAY1}&date_to=${addDays(DAY1, 1)}`);
  const created = tournaments.find((t) => t.name === TOURNAMENT);
  tournamentId = created?.id ?? '';
  categoryId = created?.categories.find((c) => c.name === CATEGORY)?.id ?? '';
  if (!tournamentId || !categoryId) throw new Error('tournament or category not created');
  const opened = await openOwnerPage(browser, world.owner);
  owner = { page: opened.page, close: () => opened.context.close() };
});

test.afterAll(async () => {
  await owner.close();
});

test('a pair paid at checkout is registered, and the owner sees it paid', async ({ page }) => {
  test.slow();
  await page.goto(landingComplexUrl(world.slug, `/tournaments/${tournamentId}`));
  await waitForStorefrontHydration(page);
  const category = page.getByRole('region', { name: CATEGORY });
  await expect(category).toBeVisible({ timeout: 15_000 });
  await category.getByRole('button', { name: BOOKING_MESSAGES.tournaments.signUp }).click();

  const id = (field: string) => `#pair_${categoryId}_${field}`;
  await page.locator(id('first_name')).fill(PAYER.firstName);
  await page.locator(id('last_name')).fill(PAYER.lastName);
  await page.locator(id('phone')).fill(PAYER.phone);
  await page.locator(id('partner_first_name')).fill(PARTNER.firstName);
  await page.locator(id('partner_last_name')).fill(PARTNER.lastName);
  await page.locator(id('partner_phone')).fill(PARTNER.phone);
  await category.getByRole('button', { name: new RegExp(pb.paySecurely) }).click();

  const preferenceId = await waitForCheckout(page);
  await payAtCheckout(page, 'approve');

  // back_urls.success: the tournament page, carrying the sign-up's token.
  await expect(page).toHaveURL(new RegExp(`/tournaments/${tournamentId}\\?token=.*registered=1`), {
    timeout: 15_000,
  });
  await expect(page.getByText(BOOKING_MESSAGES.tournaments.confirmedTitle)).toBeVisible({ timeout: 30_000 });
  payment = await paymentOfPreference(preferenceId);
  expect(payment.status).toBe('approved');

  await owner.page.goto(`/events/tournaments/${tournamentId}`);
  await expect(owner.page.getByRole('heading', { name: TOURNAMENT })).toBeVisible({ timeout: 10_000 });
  const row = owner.page.getByRole('listitem').filter({
    hasText: `${PAYER.firstName} ${PAYER.lastName} / ${PARTNER.firstName} ${PARTNER.lastName}`,
  });
  await expect(row).toContainText(tt.pairs.paid(`$${ENTRY_PESOS.toLocaleString('es-AR')}`), { timeout: 10_000 });
});

test('cancelling the category refunds the paid pair on Mercado Pago', async () => {
  await owner.page.getByRole('button', { name: tt.cancel.categoryAction }).click();
  const confirm = owner.page.getByRole('alertdialog');
  await expect(confirm.getByText(tt.cancel.categoryTitle(CATEGORY))).toBeVisible();
  await expect(confirm.getByText(tt.cancel.categoryDescriptionPaid(1))).toBeVisible();
  await confirm.getByRole('button', { name: tt.cancel.categoryConfirmLabel }).click();
  await expect(owner.page.getByText(new RegExp(`^${tt.cancel.categorySuccess(0)}`))).toBeVisible({
    timeout: 10_000,
  });

  // The whole entry goes back, service fee included.
  const refunds = await waitForRefundOf(payment.id, payment.transaction_amount);
  expect(refunds.every((r) => r.status === 'approved')).toBe(true);
});
