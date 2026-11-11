import type { Locator, Page } from '@playwright/test';
import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { createApiHelper } from '../helpers/api.helper';
import type { ApiHelper } from '../helpers/api.helper';
import { getFutureDate } from '../helpers/test-data';

/**
 * The till flows `cash.spec.ts` does not reach: voiding a movement, a
 * non-cash movement staying out of expected cash, an exact close and its
 * read-only detail page, and collecting a booking's balance at the counter.
 *
 * Same shared-complex rule as `cash.spec.ts`: every authenticated spec runs
 * against one complex, so each test starts from a closed till and closes it
 * again. Every session here is opened through the API with a $10.000 float
 * (1.000.000 centavos), so the page under test only shows this run's ledger.
 */
const OPENING_CENTAVOS = 1_000_000;

async function openCashPage(page: Page) {
  await page.goto('/cash');
  await expect(page.getByRole('heading', { name: 'Caja', exact: true })).toBeVisible({ timeout: 10_000 });
}

function expectedCard(page: Page): Locator {
  return page.getByTestId('cash-expected-card');
}

/** Records an "Ingreso" through the dialog: default category (Clases), the given method and amount in pesos. */
async function recordIncome(page: Page, pesos: string, method?: string) {
  await page.getByRole('button', { name: 'Ingreso', exact: true }).click();
  const dialog = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Ingreso' }) });
  await expect(dialog).toBeVisible({ timeout: 5_000 });

  if (method) {
    await dialog.getByLabel('Método').click();
    await page.getByRole('option', { name: method, exact: true }).click();
  }
  await dialog.getByLabel('Monto').fill(pesos);

  const response = page.waitForResponse((res) => res.url().endsWith('/movements') && res.request().method() === 'POST');
  await dialog.getByRole('button', { name: 'Ingreso', exact: true }).click();
  expect((await response).ok()).toBe(true);
  await expect(dialog).toBeHidden({ timeout: 5_000 });
}

/** Closes the open till from the UI, counting `pesos`, and returns the close dialog after the committed result. */
async function closeTill(page: Page, pesos: string): Promise<Locator> {
  await page.getByRole('button', { name: 'Cerrar caja' }).click();
  const dialog = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Cerrar caja' }) });
  await expect(dialog).toBeVisible({ timeout: 5_000 });
  await dialog.getByLabel('Efectivo contado').fill(pesos);

  const response = page.waitForResponse((res) => res.url().endsWith('/close') && res.request().method() === 'POST');
  await dialog.getByRole('button', { name: 'Cerrar caja' }).click();
  expect((await response).ok()).toBe(true);
  return dialog;
}

/**
 * Books the shared court for today at the first free hour, unpaid, so it shows
 * under "Turnos de hoy". The shared complex keeps every earlier run's bookings,
 * so an hour already taken (409) moves on to the next one.
 */
async function bookTodayUnpaid(api: ApiHelper, complexId: string, courtId: string, lastName: string) {
  const today = getFutureDate(0);
  for (let hour = 8; hour <= 21; hour++) {
    try {
      return await api.createBooking(complexId, courtId, {
        date: today,
        start_time: `${String(hour).padStart(2, '0')}:00`,
        client_first_name: 'Caja',
        client_last_name: lastName,
        client_phone: `+54911${String(Date.now()).slice(-8)}`,
      });
    } catch {
      // Taken by an earlier run; try the next hour.
    }
  }
  throw new Error('bookTodayUnpaid: no free hour left today on the shared court');
}

/** Voids the only voidable movement on the page, with a reason, from the "Anular movimiento" confirm. */
async function voidOnlyMovement(page: Page, reason: string) {
  await page.getByRole('button', { name: 'Anular', exact: true }).click();
  const dialog = page.getByRole('alertdialog').filter({ has: page.getByText('Anular movimiento') });
  await expect(dialog).toBeVisible({ timeout: 5_000 });
  await dialog.getByLabel(/^Nota/).fill(reason);

  const response = page.waitForResponse((res) => res.url().endsWith('/void') && res.request().method() === 'POST');
  await dialog.getByRole('button', { name: 'Anular', exact: true }).click();
  expect((await response).ok()).toBe(true);
  await expect(dialog).toBeHidden({ timeout: 5_000 });
}

/** A closed session's detail page shows its figures and ledger and offers no action that would change them. */
async function assertReadOnlyDetail(page: Page) {
  await expect(page.getByRole('heading', { name: 'Detalle de caja' })).toBeVisible({ timeout: 10_000 });
  const card = expectedCard(page);
  await expect(card.getByText('Sin diferencia')).toBeVisible();
  await expect(card.getByText('Contado')).toBeVisible();
  await expect(page.getByText(/Clases/)).toBeVisible();
  for (const action of ['Ingreso', 'Egreso', 'Anular', 'Cerrar caja']) {
    await expect(page.getByRole('button', { name: action, exact: true })).toHaveCount(0);
  }
}

/**
 * Adds the booking's balance from "Turnos de hoy" to the cart and charges it
 * in cash. Returns the collected amount as shown ("$150").
 */
async function collectBalance(page: Page, lastName: string): Promise<string> {
  await page.goto('/cash/sell');
  await expect(page.getByRole('heading', { name: 'Vender', exact: true })).toBeVisible({ timeout: 10_000 });

  const tile = page.getByTestId('sell-bookings-today').getByRole('button').filter({ hasText: lastName });
  await expect(tile).toBeVisible({ timeout: 10_000 });
  const remaining = ((await tile.getByText(/^Resta /).textContent()) ?? '').replace('Resta ', '').trim();
  expect(remaining).toMatch(/^\$[\d.]+$/);

  await tile.click();
  await expect(tile).toBeDisabled();
  await expect(tile.getByText('En el carrito')).toBeVisible();
  await expect(page.getByTestId('sell-balance-line').filter({ hasText: lastName })).toBeVisible();
  await expect(page.getByTestId('sell-cart-total')).toHaveText(remaining);

  const response = page.waitForResponse((res) => res.url().endsWith('/sales') && res.request().method() === 'POST');
  await page.getByRole('button', { name: `Cobrar ${remaining}`, exact: true }).click();
  expect((await response).ok()).toBe(true);

  const confirm = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Venta registrada' }) });
  await expect(confirm).toBeVisible({ timeout: 5_000 });
  const collected = confirm.getByRole('list', { name: 'Turnos cobrados' });
  await expect(collected).toContainText(lastName);
  await expect(collected).toContainText(remaining);
  await confirm.getByRole('button', { name: 'Listo' }).click();
  await expect(confirm).toBeHidden({ timeout: 5_000 });
  return remaining;
}

/** Closes counting exactly the expected $10.000 and checks the newest history row is this session, with no difference. */
async function closeExactAndCheckHistory(page: Page, sessionId: string) {
  const closeDialog = await closeTill(page, '10000');
  await expect(closeDialog.getByText('Sin diferencia')).toBeVisible({ timeout: 5_000 });
  await closeDialog.locator('button:not([data-slot="dialog-close"])', { hasText: 'Cerrar' }).click();
  await expect(page.getByText('La caja está cerrada')).toBeVisible();

  const historyRow = page.getByTestId('cash-history-list').getByRole('link').first();
  await expect(historyRow).toHaveAttribute('href', `/cash/sessions/${sessionId}`);
  await expect(historyRow).toContainText('Sin diferencia');

  // The detail page is read-only: the ledger and figures, no way to change them.
}

/** The income is marked, a reversing row names it with the reason, and neither offers "Anular" again. */
async function assertIncomeVoided(page: Page, reason: string) {
  await expect(page.getByText('Anulado', { exact: true })).toBeVisible({ timeout: 5_000 });
  await expect(page.getByText(/Anulación de Clases/)).toBeVisible();
  await expect(page.getByText(reason)).toBeVisible();
  await expect(page.getByRole('button', { name: 'Anular', exact: true })).toHaveCount(0);
}

test.describe('Cash flows', () => {
  let complexId: string;
  let courtId: string;

  // Each test starts and ends with the shared till closed.
  test.beforeEach(async () => {
    const setup = await getSharedSetup();
    ({ complexId, courtId } = setup);
    await setup.api.closeAnyOpenCashSession(complexId);
  });

  test.afterEach(async () => {
    await (await createApiHelper()).closeAnyOpenCashSession(complexId);
  });

  test('voiding a cash income writes the opposite movement and restores expected cash', async ({
    authenticatedPage: page,
  }) => {
    const api = await createApiHelper();
    await api.openCashSession(complexId, OPENING_CENTAVOS);
    await openCashPage(page);
    await expect(expectedCard(page).getByText('$10.000')).toHaveCount(2);

    await recordIncome(page, '3000');
    await expect(expectedCard(page).getByText('$13.000')).toBeVisible({ timeout: 5_000 });

    await voidOnlyMovement(page, 'Cargado por error');
    await assertIncomeVoided(page, 'Cargado por error');
    // Back to the float: opening and expected both read $10.000.
    await expect(expectedCard(page).getByText('$10.000')).toHaveCount(2);
  });

  test('a transfer income stays out of expected cash, and an exact close reads "Sin diferencia" on its detail page', async ({
    authenticatedPage: page,
  }) => {
    const api = await createApiHelper();
    const session = await api.openCashSession(complexId, OPENING_CENTAVOS);
    await openCashPage(page);

    await recordIncome(page, '5000', 'Transferencia');
    // Totals per method list the transfer; expected cash does not move.
    const transferTotal = page.getByText('Transferencia', { exact: true }).locator('..');
    // exact: the movement row itself reads "+$5.000".
    await expect(transferTotal.getByText('$5.000', { exact: true })).toBeVisible({ timeout: 5_000 });
    await expect(expectedCard(page).getByText('$10.000')).toHaveCount(2);

    await closeExactAndCheckHistory(page, session.id);

    const historyRow = page.getByTestId('cash-history-list').getByRole('link').first();
    await historyRow.click();
    await assertReadOnlyDetail(page);

    await page.getByRole('link', { name: 'Volver a Caja' }).click();
    await expect(page.getByText('La caja está cerrada')).toBeVisible({ timeout: 10_000 });
  });

  test('collecting a booking balance at the counter adds it to expected cash and clears it from today', async ({
    authenticatedPage: page,
  }) => {
    const api = await createApiHelper();
    const lastName = `Saldo${String(Date.now())}`;
    await bookTodayUnpaid(api, complexId, courtId, lastName);
    await api.openCashSession(complexId, OPENING_CENTAVOS);

    const remaining = await collectBalance(page, lastName);

    // Fully paid now: the booking leaves "Turnos de hoy".
    const today = page.getByTestId('sell-bookings-today');
    await expect(today.getByRole('button').filter({ hasText: lastName })).toHaveCount(0, { timeout: 10_000 });

    // A cash collection counts toward expected cash: float + balance.
    const pesos = (text: string) => Number(text.replace(/[$.]/g, ''));
    const expected = `$${(10_000 + pesos(remaining)).toLocaleString('es-AR')}`;
    await openCashPage(page);
    await expect(expectedCard(page).getByText(expected)).toBeVisible({ timeout: 10_000 });
  });
});
