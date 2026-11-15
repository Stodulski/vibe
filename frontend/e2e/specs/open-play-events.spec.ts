import { test, expect, type Page } from '@playwright/test';
import { landingComplexUrl, waitForStorefrontHydration } from '../helpers/landing';
import {
  addDays,
  futureWeekday,
  openOwnerPage,
  pickCalendarDate,
  provisionWorld,
  queryE2e,
  seedHeldRegistration,
  seedPaidRegistration,
  type EventsWorld,
} from '../helpers/events-fixtures';

/**
 * Canchas libres (open-play events), owner and public side
 * (odd/tasks/open-play-events.md S1, S2, S4, S5, S7, S8).
 *
 * MERCADO PAGO IS FAKE HERE. The complex carries a sealed dummy credential, so
 * a public sign-up can take its spot but there is no checkout to pay at and no
 * webhook to confirm it. Paid participants are therefore seeded straight into
 * the E2E database in the shape the webhook leaves behind (a confirmed
 * registration plus its approved `mercadopago` payment, see
 * `seedPaidRegistration`); everything the owner and the public see about them
 * is read through the real API and pages.
 *
 * One owner and complex of their own with two courts, so an event holding
 * "todas las canchas" cannot collide with any other spec. Tests run in order
 * and share the event they build.
 */
test.describe.configure({ mode: 'serial' });

const EVENT_NAME = 'Americano E2E';
const PRICE_PESOS = 5000;
const CAPACITY = 4;

// A Friday about six weeks out.
const DATE = futureWeekday(5, 40);

let world: EventsWorld;
let page: Page;
let closeContext: () => Promise<void>;
let eventId = '';

function person(n: number) {
  return { firstName: `Jugador${String(n)}`, lastName: 'Libre', phone: `+549115550${String(100 + n)}` };
}

/** The event's blocks on the grid, one per court. */
function eventBlocks(p: Page) {
  return p.getByRole('button', { name: new RegExp(`Canchas libres · ${EVENT_NAME}`) });
}

async function gotoGrid(date: string): Promise<void> {
  await page.goto(`/bookings?date=${date}`);
  await expect(page.getByRole('heading', { name: 'Reservas', exact: true })).toBeVisible({ timeout: 10_000 });
}

/** Fills the "Crear evento" dialog, leaving it open for the submit. */
async function fillEventForm(from: string, to: string): Promise<void> {
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel('Nombre del evento').fill(EVENT_NAME);
  await dialog.locator('#event-date').click();
  await pickCalendarDate(page, DATE);
  await dialog.locator('#event-start').click();
  await page.getByRole('option', { name: from, exact: true }).click();
  await dialog.locator('#event-end').click();
  await page.getByRole('option', { name: to, exact: true }).click();
  await dialog.getByLabel('Cupo').fill(String(CAPACITY));
  await dialog.getByLabel('Precio por persona').fill(String(PRICE_PESOS));
}

test.beforeAll(async ({ browser }) => {
  test.setTimeout(120_000);
  world = await provisionWorld('open-play', 2);
  const court = world.courts[0];
  if (!court) throw new Error('no court');
  // A booking on Cancha 1 at 18:00-19:00 that the first event attempt will collide with.
  await world.api.createBooking(world.complexId, court.id, {
    date: DATE,
    start_time: '18:00',
    client_first_name: 'Reserva',
    client_last_name: 'Previa',
    client_phone: '+5491155550999',
  });
  const opened = await openOwnerPage(browser, world.owner);
  page = opened.page;
  closeContext = () => opened.context.close();
});

test.afterAll(async () => {
  await closeContext();
});

test('an event over an existing booking is refused with the court named, in Spanish', async () => {
  test.slow();
  await page.goto('/events/open-play');
  await expect(page.getByRole('heading', { name: 'Eventos', exact: true })).toBeVisible({ timeout: 10_000 });
  await page.getByRole('button', { name: 'Crear evento' }).first().click();

  await fillEventForm('18:00', '20:00');
  await page.getByRole('dialog').getByRole('button', { name: 'Crear evento' }).click();

  const alert = page.getByRole('dialog').getByRole('alert');
  await expect(alert).toContainText('Ese horario no está libre');
  await expect(alert).toContainText('Ya hay una reserva en ese horario en Cancha 1.');
  await expect(alert).toContainText('Cambiá el horario o sacá esa cancha del evento.');
});

test('the owner creates the event for all courts in the free window', async () => {
  // Same dialog, the end moved so it no longer touches the booking: 20:00-22:00.
  const dialog = page.getByRole('dialog');
  await dialog.locator('#event-start').click();
  await page.getByRole('option', { name: '20:00', exact: true }).click();
  await dialog.locator('#event-end').click();
  await page.getByRole('option', { name: '22:00', exact: true }).click();
  await expect(dialog.getByLabel('Todas las canchas')).toBeChecked();
  await dialog.getByRole('button', { name: 'Crear evento' }).click();

  await expect(page.getByText('Evento creado', { exact: true })).toBeVisible({ timeout: 10_000 });
  await expect(dialog).toBeHidden();

  const card = page.getByRole('region', { name: 'Próximos' }).or(page.locator('main')).getByText(EVENT_NAME).first();
  await expect(card).toBeVisible();
  await expect(page.getByText(`0 de ${String(CAPACITY)} anotados`)).toBeVisible();
  await expect(page.getByText('2 canchas')).toBeVisible();

  const { events } = await world.api.get<{ events: { id: string; name: string }[] }>(
    `/complexes/${world.complexId}/open-play-events?date_from=${DATE}&date_to=${DATE}`,
  );
  const created = events.find((e) => e.name === EVENT_NAME);
  expect(created).toBeTruthy();
  eventId = created?.id ?? '';
});

test('the grid shows "Canchas libres · {nombre}" on every court', async () => {
  await gotoGrid(DATE);
  await expect(eventBlocks(page)).toHaveCount(2, { timeout: 10_000 });
  await expect(eventBlocks(page).first()).toHaveAccessibleName(
    new RegExp(`20:00–22:00, Canchas libres · ${EVENT_NAME}`),
  );
});

test('an owner booking inside the event is refused', async () => {
  const court = world.courts[0];
  if (!court) throw new Error('no court');
  await gotoGrid(DATE);
  await page.getByRole('button', { name: 'Nueva reserva' }).first().click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('combobox').filter({ hasText: 'Seleccioná una cancha' }).first().click();
  await page.getByRole('option', { name: court.name }).click();
  const timeSelect = dialog.getByRole('combobox').filter({ hasText: '--:--' });
  await expect(timeSelect).toBeEnabled({ timeout: 10_000 });
  await timeSelect.click();
  await page.getByRole('option', { name: '20:30', exact: true }).click();
  await dialog.getByRole('button', { name: 'Siguiente' }).click();
  await dialog.locator('#client-first-name').fill('Dentro');
  await dialog.locator('#client-last-name').fill('Evento');
  await dialog.locator('#client-phone').fill('9115550003');
  await dialog.getByRole('button', { name: 'Siguiente' }).click();
  await dialog.getByRole('button', { name: 'Nueva reserva' }).click();
  await expect(page.getByText('Ese horario es parte de un evento de canchas libres.')).toBeVisible({
    timeout: 10_000,
  });
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();

  // The API says the same thing: a 409.
  await expect(
    world.api.createBooking(world.complexId, court.id, {
      date: DATE,
      start_time: '21:00',
      client_first_name: 'Dentro',
      client_last_name: 'Evento',
      client_phone: '+5491155550003',
    }),
  ).rejects.toThrow(/createBooking failed: 409/);
});

test('the participants list shows each spot with its payment state', async () => {
  seedPaidRegistration(world.complexId, eventId, person(1), PRICE_PESOS * 100, 250);
  seedPaidRegistration(world.complexId, eventId, person(2), PRICE_PESOS * 100, 250);
  seedHeldRegistration(world.complexId, eventId, person(3), PRICE_PESOS * 100);

  await page.goto(`/events/open-play?event=${eventId}`);
  const sheet = page.getByRole('dialog');
  await expect(sheet.getByText(EVENT_NAME)).toBeVisible({ timeout: 10_000 });
  await expect(sheet.getByText('2 de 4 anotados')).toBeVisible();
  await expect(sheet.getByText('Participantes · 2 confirmados')).toBeVisible();

  const list = sheet.getByRole('list', { name: 'Participantes' });
  const first = list.getByRole('listitem').filter({ hasText: 'Jugador1 Libre' });
  await expect(first).toContainText('Pagó $5.000');
  await expect(first).toContainText('Pagó');
  const held = list.getByRole('listitem').filter({ hasText: 'Jugador3 Libre' });
  await expect(held).toContainText('Pagando');
  await expect(held).not.toContainText('Pagó $');
});

test('public: the complex page lists the event and the event page shows the sign-up form', async ({
  page: visitor,
}) => {
  await visitor.goto(landingComplexUrl(world.slug));
  await waitForStorefrontHydration(visitor);
  const section = visitor.getByRole('region', { name: 'Canchas libres' });
  await expect(section).toBeVisible({ timeout: 15_000 });
  await expect(section.getByRole('heading', { name: EVENT_NAME })).toBeVisible();
  await expect(section.getByText('Cancha 1, Cancha 2')).toBeVisible();
  await expect(section.getByText('por persona')).toBeVisible();
  await expect(section.getByText('Queda 1 lugar')).toBeVisible();

  await section.getByRole('link', { name: 'Inscribirme' }).click();
  await waitForStorefrontHydration(visitor);
  await expect(visitor).toHaveURL(new RegExp(`/c/${world.slug}/events/${eventId}`));
  await expect(visitor.getByRole('heading', { level: 1, name: EVENT_NAME })).toBeVisible({ timeout: 15_000 });
  await expect(visitor.getByText('20:00 a 22:00 hs')).toBeVisible();
  await expect(visitor.getByText('Tus datos', { exact: true })).toBeVisible();
  await expect(visitor.getByRole('button', { name: /Pagar de manera segura/ })).toBeVisible();
  // Nobody else's data is on a public page.
  await expect(visitor.getByText('Jugador1')).toHaveCount(0);
  await expect(visitor.getByText('+549115550101')).toHaveCount(0);
});

test('public: a full event closes sign-up on both pages', async ({ page: visitor }) => {
  seedPaidRegistration(world.complexId, eventId, person(4), PRICE_PESOS * 100, 250);
  // Two paid, one still being paid and this one: 4 of 4 (a held spot counts).
  await visitor.goto(landingComplexUrl(world.slug, `/events/${eventId}`));
  await waitForStorefrontHydration(visitor);
  await expect(visitor.getByRole('heading', { level: 1, name: EVENT_NAME })).toBeVisible({ timeout: 15_000 });
  await expect(visitor.getByText('Se agotaron los lugares de este evento.')).toBeVisible();
  await expect(visitor.getByText('Tus datos', { exact: true })).toHaveCount(0);

  await visitor.goto(landingComplexUrl(world.slug));
  await waitForStorefrontHydration(visitor);
  const section = visitor.getByRole('region', { name: 'Canchas libres' });
  await expect(section.getByText('Completo')).toBeVisible({ timeout: 15_000 });
  await expect(section.getByRole('link', { name: 'Inscribirme' })).toHaveCount(0);
});

test('public: an event that already started says the sign-up closed', async ({ page: visitor }) => {
  // The API only creates future events, so a second one is moved into the past in the database.
  const court = world.courts[1];
  if (!court) throw new Error('no court');
  const past = await world.post<{ event: { id: string } }>(`/complexes/${world.complexId}/open-play-events`, {
    name: 'Evento ya empezado',
    sport: 'padel',
    date: addDays(DATE, 30),
    start_time: '10:00',
    end_time: '11:00',
    court_ids: [court.id],
    capacity: 10,
    price: 300000,
  });
  queryE2e(`UPDATE open_play_events SET date = current_date - 2 WHERE id = '${past.event.id}' RETURNING id;`);

  await visitor.goto(landingComplexUrl(world.slug, `/events/${past.event.id}`));
  await waitForStorefrontHydration(visitor);
  await expect(visitor.getByRole('heading', { level: 1, name: 'Evento ya empezado' })).toBeVisible({
    timeout: 15_000,
  });
  await expect(visitor.getByText('La inscripción ya cerró.')).toBeVisible();
  await expect(visitor.getByText('Tus datos', { exact: true })).toHaveCount(0);
});

test('cancelling shows the paid count and frees the grid', async ({ page: visitor }) => {
  await page.goto(`/events/open-play?event=${eventId}`);
  const sheet = page.getByRole('dialog');
  await expect(sheet.getByText('Participantes · 3 confirmados')).toBeVisible({ timeout: 10_000 });
  await sheet.getByRole('button', { name: 'Cancelar evento' }).click();

  const confirm = page.getByRole('alertdialog');
  await expect(confirm.getByText('Cancelar el evento')).toBeVisible();
  await expect(
    confirm.getByText(
      'A las 3 personas que ya pagaron les devolvemos el pago completo por Mercado Pago, servicio incluido, y les avisamos por email si dejaron uno.',
      { exact: false },
    ),
  ).toBeVisible();
  await confirm.getByRole('button', { name: 'Cancelar evento' }).click();
  await expect(page.getByText(/^Evento cancelado/)).toBeVisible({ timeout: 10_000 });

  await gotoGrid(DATE);
  await expect(eventBlocks(page)).toHaveCount(0);
  // 20:00 is bookable again on both courts.
  const court = world.courts[0];
  if (!court) throw new Error('no court');
  const freed = await world.api.createBooking(world.complexId, court.id, {
    date: DATE,
    start_time: '20:00',
    client_first_name: 'Cancha',
    client_last_name: 'Liberada',
    client_phone: '+5491155550998',
  });
  expect(freed.status).toBe('confirmed');

  // The public event page now says so, and the complex page stops listing it.
  await visitor.goto(landingComplexUrl(world.slug, `/events/${eventId}`));
  await waitForStorefrontHydration(visitor);
  await expect(visitor.getByText('Este evento fue cancelado')).toBeVisible({ timeout: 15_000 });
  await visitor.goto(landingComplexUrl(world.slug));
  await waitForStorefrontHydration(visitor);
  await expect(visitor.getByRole('region', { name: 'Canchas libres' })).toHaveCount(0);
});
