/* eslint-disable max-lines -- one serial story (create, draw, schedule, results, fee, cancel) that shares its tournament */
import { test, expect, type Locator, type Page } from '@playwright/test';
import { ES_AR } from '../../src/shared/i18n/es_AR';
import { landingComplexUrl, waitForStorefrontHydration } from '../helpers/landing';
import {
  addDays,
  futureWeekday,
  openOwnerPage,
  pickCalendarDate,
  provisionWorld,
  seedPaidPair,
  type EventsWorld,
} from '../helpers/events-fixtures';

/**
 * Torneos, owner and public side (odd/tasks/tournaments.md S1-S9).
 *
 * The paid pairs are setup here, not the subject: they are seeded straight into
 * the E2E database in the shape the webhook leaves behind (the payer's
 * confirmed registration with its approved `mercadopago` payment plus the
 * partner row, see `seedPaidPair`). Paying a pair through the fake Mercado Pago,
 * and its refund, is `tournament-payments.spec.ts`. The tournament itself, its draws,
 * schedule, results, court fees and cancellation all go through the real owner
 * app and API, and the public page is read as a visitor.
 *
 * One owner and complex of their own (two courts, an open till), so nothing
 * here collides with the shared complex. Tests run in order and share state.
 */
test.describe.configure({ mode: 'serial' });

const TOURNAMENT = 'Copa E2E';
const GROUPS = '6ta';
const KNOCKOUT = 'Mixto';
const GROUPS_PAIRS = 8;
const KNOCKOUT_PAIRS = 5;
const ENTRY_GROUPS = 8000;
const ENTRY_KNOCKOUT = 6000;
const COURT_FEE = 2000;

// Saturday and Sunday about six weeks out: nothing else in the run touches them.
const DAY1 = futureWeekday(6, 40);
const DAY2 = addDays(DAY1, 1);

let world: EventsWorld;
let page: Page;
let closeContext: () => Promise<void>;
let tournamentId = '';
const categoryIds: Record<string, string> = {};

async function seedPairs(category: string, prefix: string, count: number, price: number): Promise<void> {
  const categoryId = categoryIds[category];
  if (!categoryId) throw new Error(`no category ${category}`);
  for (let i = 1; i <= count; i++) {
    await seedPaidPair(
      world.complexId,
      categoryId,
      {
        firstName: 'Ana',
        lastName: `${prefix}${String(i)}`,
        phone: `+54911555${prefix === 'Zeta' ? '1' : '2'}${String(100 + i)}`,
      },
      {
        firstName: 'Beto',
        lastName: `${prefix}x${String(i)}`,
        phone: `+54911555${prefix === 'Zeta' ? '3' : '4'}${String(100 + i)}`,
      },
      price * 100,
    );
  }
}

async function openTournament(): Promise<void> {
  await page.goto(`/events/tournaments/${tournamentId}`);
  await expect(page.getByRole('heading', { name: TOURNAMENT })).toBeVisible({ timeout: 10_000 });
}

async function chooseCategory(name: string): Promise<void> {
  await page.getByRole('group', { name: 'Categorías' }).getByRole('button', { name, exact: true }).click();
}

async function chooseSection(name: string): Promise<void> {
  await page
    .getByRole('group', { name: 'Secciones de la categoría' })
    .getByRole('button', { name, exact: true })
    .click();
}

/** Draws the open category through the confirm dialog. */
async function draw(): Promise<void> {
  await page.getByRole('button', { name: 'Sortear', exact: true }).click();
  const confirm = page.getByRole('alertdialog');
  await expect(confirm.getByText('Sortear la categoría')).toBeVisible();
  await confirm.getByRole('button', { name: 'Sortear', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Sortear de nuevo' })).toBeVisible({ timeout: 10_000 });
}

test.beforeAll(async ({ browser }) => {
  test.setTimeout(120_000);
  world = await provisionWorld('tournaments', 2);
  const court = world.courts[0];
  if (!court) throw new Error('no court');
  // A booking at 12:00 on Cancha 1 the schedule conflict test collides with.
  await world.api.createBooking(world.complexId, court.id, {
    date: DAY1,
    start_time: '12:00',
    client_first_name: 'Reserva',
    client_last_name: 'Previa',
    client_phone: '+5491155550999',
  });
  // The court fee lands in the till, so one must be open.
  await world.api.openCashSession(world.complexId, 1_000_000);
  const opened = await openOwnerPage(browser, world.owner);
  page = opened.page;
  closeContext = () => opened.context.close();
});

test.afterAll(async () => {
  await closeContext();
});

test('the owner creates a tournament with a groups category and an elimination category', async () => {
  test.slow();
  await page.goto('/events/tournaments');
  await expect(page.getByRole('heading', { name: 'Eventos', exact: true })).toBeVisible({ timeout: 10_000 });
  await page.getByRole('button', { name: 'Crear torneo' }).first().click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByText('Cada categoría tiene su cupo, su precio por pareja y su formato.')).toBeVisible();

  await dialog.getByLabel('Nombre del torneo').fill(TOURNAMENT);
  await dialog.locator('#tournament-start').click();
  await pickCalendarDate(page, DAY1);
  await dialog.locator('#tournament-end').click();
  await pickCalendarDate(page, DAY2);

  await dialog.getByRole('button', { name: 'Siguiente' }).click();

  // Step 2, category 1: groups of 4 where 2 advance, 60 minute matches, a court fee.
  await dialog.getByRole('button', { name: 'Agregar categoría' }).click();
  await dialog.locator('#category-0-name').fill(GROUPS);
  await dialog.locator('#category-0-capacity').fill(String(GROUPS_PAIRS));
  await dialog.locator('#category-0-price').fill(String(ENTRY_GROUPS));
  await dialog.locator('#category-0-court-fee').fill(String(COURT_FEE));
  await dialog.getByRole('button', { name: 'Siguiente' }).click();
  await expect(dialog.getByLabel('Parejas por zona')).toBeVisible();
  await dialog.getByLabel('Parejas por zona').fill('4');
  await dialog.getByLabel('Pasan por zona').fill('2');
  await dialog.getByLabel('Duración de cada partido (minutos)').fill('60');
  await dialog.getByRole('button', { name: 'Guardar categoría' }).click();

  // Category 2: single elimination.
  await dialog.getByRole('button', { name: 'Agregar categoría' }).click();
  await dialog.locator('#category-1-name').fill(KNOCKOUT);
  await dialog.locator('#category-1-format').click();
  await page.getByRole('option', { name: 'Eliminación directa' }).click();
  await dialog.locator('#category-1-capacity').fill('8');
  await dialog.locator('#category-1-price').fill(String(ENTRY_KNOCKOUT));
  await dialog.getByRole('button', { name: 'Siguiente' }).click();
  await dialog.getByRole('button', { name: 'Guardar categoría' }).click();
  await dialog.getByRole('button', { name: 'Siguiente' }).click();

  // Step 3: the review lists both categories before anything is sent.
  await expect(dialog.getByText(GROUPS, { exact: true })).toBeVisible();
  await expect(dialog.getByText(KNOCKOUT, { exact: true })).toBeVisible();

  await dialog.getByRole('button', { name: 'Crear torneo' }).click();
  await expect(page.getByText('Torneo creado', { exact: true })).toBeVisible({ timeout: 10_000 });
  await expect(dialog).toBeHidden();

  const card = page.getByRole('link', { name: new RegExp(TOURNAMENT) });
  await expect(card).toBeVisible();
  await expect(card).toContainText(`${GROUPS}, ${KNOCKOUT}`);

  const { tournaments } = await world.api.get<{
    tournaments: { id: string; name: string; categories: { id: string; name: string }[] }[];
  }>(`/complexes/${world.complexId}/tournaments?date_from=${DAY1}&date_to=${DAY2}`);
  const created = tournaments.find((t) => t.name === TOURNAMENT);
  expect(created).toBeTruthy();
  tournamentId = created?.id ?? '';
  for (const c of created?.categories ?? []) categoryIds[c.name] = c.id;
  expect(Object.keys(categoryIds).sort()).toEqual([GROUPS, KNOCKOUT]);
});

test('the paid pairs show with their payment, seed and contact', async () => {
  await seedPairs(GROUPS, 'Zeta', GROUPS_PAIRS, ENTRY_GROUPS);
  await seedPairs(KNOCKOUT, 'Omega', KNOCKOUT_PAIRS, ENTRY_KNOCKOUT);

  await openTournament();
  await expect(page.getByText(`${String(GROUPS_PAIRS)} de ${String(GROUPS_PAIRS)} parejas`)).toBeVisible();
  const row = page.getByRole('listitem').filter({ hasText: 'Ana Zeta1 / Beto Zetax1' });
  await expect(row).toContainText('Pagó');
  await expect(row).toContainText('Pagó $8.000');
  await expect(row.getByRole('link', { name: 'Llamar Ana Zeta1' })).toBeVisible();
  await expect(page.getByText('Inscripción abierta').first()).toBeVisible();
});

test('public: before the draw the page shows the categories, the sign-up and no phones', async ({ page: visitor }) => {
  await visitor.goto(landingComplexUrl(world.slug));
  await waitForStorefrontHydration(visitor);
  const section = visitor.getByRole('region', { name: 'Torneos' });
  await expect(section).toBeVisible({ timeout: 15_000 });
  await section.getByRole('link', { name: /Ver torneo/ }).click();
  await waitForStorefrontHydration(visitor);
  await expect(visitor).toHaveURL(new RegExp(`/c/${world.slug}/tournaments/${tournamentId}`));
  await expect(visitor.getByRole('heading', { name: TOURNAMENT })).toBeVisible({ timeout: 15_000 });
  await expect(visitor.getByText('Zonas y cuadro').first()).toBeVisible();
  await expect(visitor.getByText('Eliminación directa').first()).toBeVisible();
  // 6ta has all 8 pairs paid: full. Mixto has 5 of 8: three spots and the sign-up.
  const full = visitor.getByRole('region', { name: GROUPS });
  await expect(full.getByText('Se agotaron los lugares de esta categoría.')).toBeVisible();
  await expect(full.getByRole('button', { name: 'Inscribir mi pareja' })).toHaveCount(0);
  const open = visitor.getByRole('region', { name: KNOCKOUT });
  await expect(open.getByText(/\$[\d.]+ por pareja/)).toBeVisible();
  await expect(open.getByText('Quedan 3 lugares')).toBeVisible();
  await open.getByRole('button', { name: 'Inscribir mi pareja' }).click();
  await expect(visitor.getByText('Datos de tu pareja')).toBeVisible();
  await expect(visitor.getByText('Inscripción de la pareja', { exact: true })).toBeVisible();
  await expect(visitor.getByText('Tiene que ser distinto del tuyo.')).toBeVisible();
  // Names are public, phones never are.
  await expect(visitor.locator('body')).not.toContainText('+54911555');
  await expect(visitor.getByText('Zeta1')).toHaveCount(0);
});

test('drawing the groups category builds zones with standings and a bracket', async () => {
  await openTournament();
  await chooseCategory(GROUPS);
  await draw();

  await chooseSection('Zonas');
  for (const zone of ['A', 'B']) {
    const table = page.getByRole('table', { name: `Posiciones de la zona ${zone}` });
    await expect(table).toBeVisible();
    // Four pairs per zone, the best two marked as qualifying.
    await expect(table.getByRole('row')).toHaveCount(5);
    await expect(table.getByText('Clasifica')).toHaveCount(2);
  }
  await chooseSection('Cuadro');
  await expect(page.getByText('Semifinales')).toBeVisible();
  await expect(page.getByText('Final', { exact: true })).toBeVisible();
  await expect(page.getByText('1º de la zona A').first()).toBeVisible();

  // The draw closed the category's sign-up and cannot be done from scratch any more.
  await chooseSection('Parejas');
  await expect(page.getByText('Sorteo hecho').first()).toBeVisible();
  await expect(page.getByRole('button', { name: 'Sortear de nuevo' })).toBeVisible();
});

test('drawing the elimination category fills the bracket with byes for the top seeds', async () => {
  await openTournament();
  await chooseCategory(KNOCKOUT);
  await draw();
  await chooseSection('Cuadro');
  await expect(page.getByText('Cuartos de final')).toBeVisible();
  await expect(page.getByText('Libre', { exact: true })).toHaveCount(3);
});

test('scheduling a match puts it on the grid; a booking, or another match, in the way is refused', async () => {
  test.slow();
  await openTournament();
  await chooseCategory(GROUPS);
  await chooseSection('Partidos');

  const first = page.getByRole('listitem').filter({ has: page.getByRole('button', { name: 'Programar' }) });
  await first.first().getByRole('button', { name: 'Programar' }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByText('Por defecto 60 minutos, según la categoría.')).toBeVisible();

  async function fill(courtName: string, day: string, time: string) {
    await dialog.locator('#match-court').click();
    await page.getByRole('option', { name: courtName }).click();
    await dialog.locator('#match-date').click();
    await pickCalendarDate(page, day);
    await dialog.locator('#match-time').click();
    await page.getByRole('option', { name: time, exact: true }).click();
  }

  // 12:00 on Cancha 1 is the booking seeded in beforeAll.
  await fill('Cancha 1', DAY1, '12:00');
  await dialog.getByRole('button', { name: 'Guardar horario' }).click();
  await expect(dialog).toContainText('Ya hay una reserva en ese horario en Cancha 1.');

  await dialog.locator('#match-time').click();
  await page.getByRole('option', { name: '10:00', exact: true }).click();
  await dialog.getByRole('button', { name: 'Guardar horario' }).click();
  await expect(page.getByText('Partido programado')).toBeVisible({ timeout: 10_000 });
  // The court name is kept on one line with no-break spaces (keepTogether), hence `\s`.
  await expect(page.getByText(/Cancha\s1 · .* · 10:00/).first()).toBeVisible();

  // A second match overlapping 10:00-11:00 on the same court.
  await page
    .getByRole('listitem')
    .filter({ has: page.getByRole('button', { name: 'Programar' }) })
    .first()
    .getByRole('button', { name: 'Programar' })
    .click();
  await fill('Cancha 1', DAY1, '10:30');
  await dialog.getByRole('button', { name: 'Guardar horario' }).click();
  await expect(dialog).toContainText('Ya hay otro partido de torneo en ese horario en Cancha 1.');
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();

  await page.goto(`/bookings?date=${DAY1}`);
  await expect(page.getByRole('heading', { name: 'Reservas', exact: true })).toBeVisible({ timeout: 10_000 });
  // EventBlockBar: "{cancha}, 10:00 a 11:00, Torneo · {nombre}" (ranges are spelled with "a").
  await expect(page.getByRole('button', { name: /Cancha\s1, 10:00 a 11:00, Torneo · / })).toBeVisible({
    timeout: 10_000,
  });
});

let winnerOnly = '';
let scoredWinner = '';

/** The two pair names of the open result dialog ("A vs B"), read from its description. */
async function dialogPairs(dialog: Locator): Promise<[string, string]> {
  const text = (await dialog.locator('[id$="description"], p').filter({ hasText: ' vs ' }).first().innerText()).trim();
  const [a = '', b = ''] = text.split(' vs ');
  return [a.trim(), b.trim()];
}

test('a winner-only result and a scored result are loaded and the standings follow', async () => {
  test.slow();
  await openTournament();
  await chooseCategory(GROUPS);
  await chooseSection('Partidos');

  // Winner only: no sets.
  await page.getByRole('button', { name: 'Cargar resultado' }).first().click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByText('Resultado del partido')).toBeVisible();
  const [a1] = await dialogPairs(dialog);
  await dialog.getByRole('button', { name: a1, exact: true }).click();
  await dialog.getByRole('button', { name: 'Guardar resultado' }).click();
  await expect(dialog).toBeHidden({ timeout: 10_000 });
  winnerOnly = a1;
  await expect(page.getByText(`Ganó ${a1}`)).toBeVisible();

  // Scored: 6-4 6-3 for the first pair.
  await page.getByRole('button', { name: 'Cargar resultado' }).first().click();
  await dialog.getByRole('button', { name: 'Cargar resultado por sets' }).click();
  const [a2] = await dialogPairs(dialog);
  const inputs = dialog.getByRole('spinbutton');
  for (const [i, v] of ['6', '4', '6', '3'].entries()) await inputs.nth(i).fill(v);
  await dialog.getByRole('button', { name: 'Guardar resultado' }).click();
  await expect(dialog).toBeHidden({ timeout: 10_000 });
  scoredWinner = a2;
  await expect(page.getByText('6-4 6-3')).toBeVisible();

  // Both winners have one match played and one won in their zone table.
  await chooseSection('Zonas');
  for (const name of [winnerOnly, scoredWinner]) {
    const cells = page.getByRole('row').filter({ hasText: name }).getByRole('cell');
    await expect(cells.nth(2)).toHaveText('1');
    await expect(cells.nth(3)).toHaveText('1');
  }
});

test('the winner of a knockout match advances to the next round', async () => {
  await openTournament();
  await chooseCategory(KNOCKOUT);
  await chooseSection('Partidos');
  const semiBefore = await page.getByText('A definir').count();
  expect(semiBefore).toBeGreaterThan(0);

  await page.getByRole('button', { name: 'Cargar resultado' }).first().click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('button', { name: 'Cargar resultado por sets' }).click();
  const [winner] = await dialogPairs(dialog);
  const inputs = dialog.getByRole('spinbutton');
  for (const [i, v] of ['6', '2', '6', '1'].entries()) await inputs.nth(i).fill(v);
  await dialog.getByRole('button', { name: 'Guardar resultado' }).click();
  await expect(dialog).toBeHidden({ timeout: 10_000 });

  await expect(page.getByText('6-2 6-1')).toBeVisible();
  // The winner now appears in the played match and in the semifinal it moved to.
  await expect(page.getByRole('listitem').filter({ hasText: winner })).toHaveCount(2);
  await expect(page.getByText('A definir')).toHaveCount(semiBefore - 1);
});

test('a court fee is charged with the open till and shows up in Caja', async () => {
  await openTournament();
  await chooseCategory(GROUPS);
  await chooseSection('Partidos');
  await expect(page.getByText('Cancha: $2.000 por pareja').first()).toBeVisible();

  await page
    .getByRole('listitem')
    .filter({ hasText: `Ganó ${winnerOnly}` })
    .getByRole('button', { name: `Cobrar la cancha a ${winnerOnly}` })
    .click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByText('Cobrar la cancha')).toBeVisible();
  await dialog.getByRole('button', { name: 'Cobrar $2.000' }).click();
  await expect(page.getByText('Cobro registrado en la caja')).toBeVisible({ timeout: 10_000 });
  await expect(page.getByText('Pagó $2.000').first()).toBeVisible();
  await expect(page.getByText(/Cobrado \$2\.000 · Pendiente/)).toBeVisible();

  await page.goto('/cash');
  await expect(page.getByRole('heading', { name: 'Caja', exact: true })).toBeVisible({ timeout: 10_000 });
  await expect(page.getByText(/Cancha de torneo/).first()).toBeVisible();
});

test('public: after the draw the page shows zones, bracket, schedule and results, never phones', async ({
  page: visitor,
}) => {
  await visitor.goto(landingComplexUrl(world.slug, `/tournaments/${tournamentId}`));
  await waitForStorefrontHydration(visitor);
  const category = visitor.getByRole('region', { name: GROUPS });
  await expect(category).toBeVisible({ timeout: 15_000 });
  await expect(category.getByText('Zonas', { exact: true })).toBeVisible();
  await expect(category.getByRole('table', { name: 'Posiciones de la zona A' })).toBeVisible();
  await expect(category.getByRole('table', { name: 'Posiciones de la zona B' })).toBeVisible();
  await expect(category.getByText('Cuadro', { exact: true })).toBeVisible();
  // A zone's matches are listed inside the zone, one heading per round ("Fecha 1").
  await expect(category.getByRole('heading', { name: 'Fecha 1' }).first()).toBeVisible();
  await expect(category.getByText(/Cancha\s1/).first()).toBeVisible();
  await expect(category.getByText('Ganó').first()).toBeVisible();
  await expect(category.getByText('6-4 6-3')).toBeVisible();

  const knockout = visitor.getByRole('region', { name: KNOCKOUT });
  await expect(knockout.getByText('Cuartos de final')).toBeVisible();
  await expect(knockout.getByText('Libre').first()).toBeVisible();

  await expect(visitor.locator('body')).not.toContainText('+54911555');
});

test('cancelling a category shows the paid pairs it refunds', async () => {
  await openTournament();
  await chooseCategory(KNOCKOUT);
  await page.getByRole('button', { name: 'Cancelar categoría' }).click();
  const confirm = page.getByRole('alertdialog');
  await expect(confirm.getByText('Cancelar la categoría Mixto')).toBeVisible();
  await expect(confirm).toContainText(ES_AR.tournaments.cancel.categoryDescriptionPaid(5));
  await confirm.getByRole('button', { name: ES_AR.tournaments.cancel.categoryConfirmLabel }).click();
  await expect(page.getByText(/^Categoría cancelada/)).toBeVisible({ timeout: 10_000 });
  await expect(
    page.getByRole('group', { name: 'Categorías' }).getByRole('button', { name: /Mixto · Cancelada/ }),
  ).toBeVisible();
});
