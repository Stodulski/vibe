import type { Page } from '@playwright/test';
import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { createApiHelper } from '../helpers/api.helper';

/**
 * The shared complex's cash session is exactly as shared as its bookings and
 * its one-complex-per-account (see `shared-setup.ts`, `TST-07`): every
 * authenticated spec in this project runs serially against the same
 * complex, so this suite must never leave a session open behind it — a
 * left-open session would make `useCashSession`'s 404-means-closed check
 * answer 200 for the very first thing the NEXT run of this spec (or a
 * rerun after a crash) expects to find closed.
 *
 * The session HISTORY is shared the same way and is never reset between
 * runs — every rerun/retry of this spec adds one more closed session with
 * the same $500 shortfall to it. A page-wide `getByText(/Faltante/)` or a
 * bare `$8.000` locator can therefore match several elements (Playwright's
 * strict mode then refuses to resolve it) or silently match a stale prior
 * row instead of this run's own session. Every assertion below is scoped to
 * the specific region it is about instead: the open-session summary card,
 * the close dialog, and the newest (first) history row.
 */
async function openCashPage(page: Page) {
  await page.goto('/cash');
  await expect(page.getByRole('heading', { name: 'Caja', exact: true })).toBeVisible({ timeout: 10_000 });
}

/** Opens the till and records a $2.000 expense against a $10.000 float, ending at $8.000 expected cash. */
async function openTillAndRecordExpense(page: Page) {
  await page.getByRole('button', { name: 'Abrir caja' }).click();
  const openDialog = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Abrir caja' }) });
  await expect(openDialog).toBeVisible({ timeout: 5_000 });

  await openDialog.getByLabel('Monto inicial').fill('10000');
  const openResponse = page.waitForResponse(
    (res) => res.url().endsWith('/cash-sessions') && res.request().method() === 'POST',
  );
  await openDialog.getByRole('button', { name: 'Abrir caja' }).click();
  expect((await openResponse).ok()).toBe(true);
  await expect(openDialog).toBeHidden({ timeout: 5_000 });

  const summary = page.getByTestId('cash-expected-card');
  await expect(summary.getByText('Efectivo esperado')).toBeVisible();

  await page.getByRole('button', { name: 'Egreso' }).click();
  const expenseDialog = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Egreso' }) });
  await expect(expenseDialog).toBeVisible({ timeout: 5_000 });

  await expenseDialog.getByLabel('Monto').fill('2000');
  const movementResponse = page.waitForResponse(
    (res) => res.url().endsWith('/movements') && res.request().method() === 'POST',
  );
  await expenseDialog.getByRole('button', { name: 'Egreso' }).click();
  expect((await movementResponse).ok()).toBe(true);
  await expect(expenseDialog).toBeHidden({ timeout: 5_000 });

  // Scoped to the summary card — a closed history row can carry the same
  // figure ($8.000, or the same shortfall) from a previous run.
  await expect(summary.getByText('$8.000')).toBeVisible({ timeout: 5_000 });
}

/** Closes with a $500 shortfall (7500 counted - 8000 expected) and asserts the committed result. */
async function closeWithShortfall(page: Page) {
  await page.getByRole('button', { name: 'Cerrar caja' }).click();
  const closeDialog = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Cerrar caja' }) });
  await expect(closeDialog).toBeVisible({ timeout: 5_000 });

  await closeDialog.getByLabel('Efectivo contado').fill('7500');
  // Asserted by amount (not just the "Faltante" label) so a schema/
  // formatting regression that produced the wrong figure would still fail.
  await expect(closeDialog.getByText('Faltante: $500')).toBeVisible();

  const closeResponse = page.waitForResponse(
    (res) => res.url().endsWith('/close') && res.request().method() === 'POST',
  );
  await closeDialog.getByRole('button', { name: 'Cerrar caja' }).click();
  expect((await closeResponse).ok()).toBe(true);

  // Closed result shown inline before returning to the closed state. Two
  // "Cerrar" buttons exist in the dialog at this point (the sr-only ✕ close
  // and the result view's own submit) — excluded by `data-slot`.
  await expect(closeDialog.getByText('Faltante: $500')).toBeVisible({ timeout: 5_000 });
  await closeDialog.locator('button:not([data-slot="dialog-close"])', { hasText: 'Cerrar' }).click();
  await expect(closeDialog).toBeHidden({ timeout: 5_000 });
}

test.describe('Cash', () => {
  let complexId: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
    // Robust against a previous crashed run (or a previous spec file) having
    // left a session open — this describe block always starts from closed.
    await setup.api.closeAnyOpenCashSession(complexId);
  });

  test.afterAll(async () => {
    if (!complexId) return;
    const api = await createApiHelper();
    await api.closeAnyOpenCashSession(complexId);
  });

  test('owner can open the till, record an expense, watch expected cash drop, and close with a difference', async ({
    authenticatedPage: page,
  }) => {
    await openCashPage(page);
    await expect(page.getByText('La caja está cerrada')).toBeVisible();

    await openTillAndRecordExpense(page);
    await closeWithShortfall(page);

    // ─── Back to the closed state, with the session now in history ───
    await expect(page.getByText('La caja está cerrada')).toBeVisible();
    const newestHistoryEntry = page.getByTestId('cash-history-list').getByRole('link').first();
    await expect(newestHistoryEntry).toContainText('Faltante: $500');
  });
});
