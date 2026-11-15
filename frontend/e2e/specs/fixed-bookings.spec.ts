import { test, expect, type Page } from '@playwright/test';
import { addDays, futureWeekday, openOwnerPage, provisionWorld, type EventsWorld } from '../helpers/events-fixtures';

/**
 * Turnos fijos (weekly fixed bookings), owner side, end to end through the
 * owner app (odd/tasks/recurring-bookings.md S2b, S3, S5, S6, S7).
 *
 * A series books nothing ahead: every matching future date is a "Bloqueo por
 * turno fijo" on the grid until a job turns the day's date into a booking.
 * All dates here are Mondays months ahead, so no job ever settles them.
 *
 * One owner and complex of its own (helpers/events-fixtures.ts): the series
 * holds its hour on every court-week, which must not collide with the other
 * authenticated specs sharing `complejo-e2e-shared`. The tests run in order and
 * share the series they build.
 */
test.describe.configure({ mode: 'serial' });

const CLIENT = { first: 'Serie', last: 'Fijo', phone: '9115550001' };
const BLOCK = /Turno fijo · Serie Fijo/;

// Monday, ~4 months out: far from the 06:00 "already due" window of a new series.
const FIRST = futureWeekday(1, 120);
const SECOND = addDays(FIRST, 7);
const THIRD = addDays(FIRST, 14);
const FOURTH = addDays(FIRST, 21);
const FIFTH = addDays(FIRST, 28);

let world: EventsWorld;
let page: Page;
let closeContext: () => Promise<void>;

async function gotoDay(date: string): Promise<void> {
  await page.goto(`/bookings?date=${date}`);
  await expect(page.getByRole('heading', { name: 'Reservas', exact: true })).toBeVisible({ timeout: 10_000 });
}

/** The grid's red dashed block of a date no booking exists for yet. */
function fixedBlock() {
  return page.getByRole('button', { name: BLOCK });
}

test.beforeAll(async ({ browser }) => {
  test.setTimeout(120_000);
  world = await provisionWorld('fixed-bookings', 1);
  const opened = await openOwnerPage(browser, world.owner);
  page = opened.page;
  closeContext = () => opened.context.close();
});

test.afterAll(async () => {
  await closeContext();
});

test('the owner creates a weekly fixed booking from the new booking dialog', async () => {
  test.slow();
  await gotoDay(FIRST);
  await page.getByRole('button', { name: 'Nueva reserva' }).first().click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();

  // Step 1: court and 19:00, repeating every week with no end.
  await dialog.getByRole('combobox').filter({ hasText: 'Seleccioná una cancha' }).first().click();
  await page.getByRole('option', { name: world.courts[0]?.name ?? '' }).click();
  const timeSelect = dialog.getByRole('combobox').filter({ hasText: '--:--' });
  await expect(timeSelect).toBeEnabled({ timeout: 10_000 });
  await timeSelect.click();
  await page.getByRole('option', { name: '19:00', exact: true }).click();
  await dialog.getByLabel('Repetir todas las semanas').click();
  await expect(dialog.getByText('Hasta cuándo')).toBeVisible();
  await expect(dialog.getByRole('combobox').filter({ hasText: 'Sin fecha de fin' })).toBeVisible();
  await dialog.getByRole('button', { name: 'Siguiente' }).click();

  // Step 2: the client.
  await dialog.locator('#client-first-name').fill(CLIENT.first);
  await dialog.locator('#client-last-name').fill(CLIENT.last);
  await dialog.locator('#client-phone').fill(CLIENT.phone);
  await dialog.getByRole('button', { name: 'Siguiente' }).click();

  // Step 3: nothing to pay up front ("Cada turno se cobra como una reserva normal").
  const created = page.waitForResponse((r) => r.url().endsWith('/booking-series') && r.request().method() === 'POST');
  await dialog.getByRole('button', { name: 'Nueva reserva' }).click();
  expect((await created).status()).toBe(201);

  await expect(page.getByText('Turno fijo creado: todos los lunes a las 19:00')).toBeVisible({ timeout: 10_000 });
  await expect(dialog).toBeHidden();
});

test('every future matching date shows "Turno fijo · {cliente}" on the grid', async () => {
  await gotoDay(THIRD);
  await expect(fixedBlock()).toBeVisible({ timeout: 10_000 });
  await expect(fixedBlock()).toHaveAccessibleName(/19:00 a 20:30, Turno fijo · Serie Fijo/);

  // The block opens a sheet that says which weekly booking it is and offers the two actions.
  await fixedBlock().click();
  const sheet = page.getByRole('dialog');
  await expect(sheet.getByText('Turno fijo · todos los lunes a las 19:00')).toBeVisible();
  await expect(sheet.getByText('Serie Fijo')).toBeVisible();
  await expect(sheet.getByRole('button', { name: 'Cancelar solo esta fecha' })).toBeVisible();
  await expect(sheet.getByRole('button', { name: 'Cancelar turno fijo' })).toBeVisible();
});

test('"Cancelar solo esta fecha" frees that date and leaves the others blocked', async () => {
  await gotoDay(THIRD);
  await fixedBlock().click();
  await page.getByRole('dialog').getByRole('button', { name: 'Cancelar solo esta fecha' }).click();
  await expect(page.getByText(/^Fecha liberada: /)).toBeVisible({ timeout: 10_000 });
  await expect(fixedBlock()).toHaveCount(0);

  // Reload: the freed date stays free, the one after it is still held.
  await gotoDay(THIRD);
  await expect(fixedBlock()).toHaveCount(0);
  await gotoDay(FOURTH);
  await expect(fixedBlock()).toBeVisible({ timeout: 10_000 });
});

test('a booking over a blocked occurrence is refused, and the freed date takes it', async () => {
  const court = world.courts[0];
  if (!court) throw new Error('no court');
  const booking = (date: string, last: string) =>
    world.api.createBooking(world.complexId, court.id, {
      date,
      start_time: '19:00',
      client_first_name: 'Choca',
      client_last_name: last,
      client_phone: '+5491155550002',
    });

  // The owner dialog offers the hour but the server refuses it, and the app says why.
  await gotoDay(FOURTH);
  await page.getByRole('button', { name: 'Nueva reserva' }).first().click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('combobox').filter({ hasText: 'Seleccioná una cancha' }).first().click();
  await page.getByRole('option', { name: court.name }).click();
  const timeSelect = dialog.getByRole('combobox').filter({ hasText: '--:--' });
  await expect(timeSelect).toBeEnabled({ timeout: 10_000 });
  await timeSelect.click();
  await page.getByRole('option', { name: '19:00', exact: true }).click();
  await dialog.getByRole('button', { name: 'Siguiente' }).click();
  await dialog.locator('#client-first-name').fill('Choca');
  await dialog.locator('#client-last-name').fill('Fijo');
  await dialog.locator('#client-phone').fill('9115550002');
  await dialog.getByRole('button', { name: 'Siguiente' }).click();
  await dialog.getByRole('button', { name: 'Nueva reserva' }).click();
  await expect(
    page.getByText('Ese horario es de un turno fijo. Cancelá esa fecha del turno fijo para usarlo.'),
  ).toBeVisible({ timeout: 10_000 });
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();

  // Straight to the API the same date is a 409.
  await expect(booking(FOURTH, 'Fijo')).rejects.toThrow(/createBooking failed: 409/);

  // THIRD was freed with "Cancelar solo esta fecha": bookable again.
  const freed = await booking(THIRD, 'Libre');
  expect(freed.status).toBe('confirmed');
});

test('"Cancelar turno fijo" ends the series from that date on', async () => {
  await gotoDay(FOURTH);
  await fixedBlock().click();
  await page.getByRole('dialog').getByRole('button', { name: 'Cancelar turno fijo' }).click();
  const confirm = page.getByRole('alertdialog');
  await expect(confirm.getByText('Cancelar el turno fijo')).toBeVisible();
  await expect(
    confirm.getByText(
      'Esta fecha y las siguientes quedan libres. Si alguna ya tiene su reserva sin pago, se cancela; si tiene un pago, se mantiene.',
    ),
  ).toBeVisible();
  await confirm.getByRole('button', { name: 'Cancelar turno fijo' }).click();
  await expect(page.getByText('Turno fijo cancelado', { exact: true })).toBeVisible({ timeout: 10_000 });

  // From FOURTH on nothing is held any more.
  for (const date of [FOURTH, FIFTH]) {
    await gotoDay(date);
    await expect(fixedBlock()).toHaveCount(0);
  }
});

// PRODUCT BUG (found by this spec): ending a series from a date sets status
// 'ended' next to the new ends_on, but booking_series_occurrences keeps only
// `status = 'active'` series (backend/db/migrations/027_booking_series.sql:174),
// so the dates BEFORE the one cancelled stop holding their hours too, and the
// daily job (which reads the same function) never turns them into bookings.
// recurring-bookings.md S6 says only the dates from `from_date` on are freed.
// `test.fail` keeps the suite green while the bug stands and turns red the day
// it is fixed: then drop the annotation.
test('the dates before the cancelled one keep holding their hours', async () => {
  test.fail(true, 'booking_series_occurrences ignores an ended series even for dates up to its ends_on');
  await gotoDay(SECOND);
  await expect(fixedBlock()).toBeVisible({ timeout: 10_000 });
  await gotoDay(FIRST);
  await expect(fixedBlock()).toBeVisible({ timeout: 10_000 });
});
