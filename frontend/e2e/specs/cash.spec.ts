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
 */
async function openCashPage(page: Page) {
  await page.goto('/cash');
  await expect(page.getByRole('heading', { name: 'Caja', exact: true })).toBeVisible({ timeout: 10_000 });
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

    // ─── Closed state ───
    await expect(page.getByText('La caja está cerrada')).toBeVisible();

    // ─── Open the till ───
    await page.getByRole('button', { name: 'Abrir caja' }).click();
    const openDialog = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Abrir caja' }) });
    await expect(openDialog).toBeVisible({ timeout: 5_000 });

    await openDialog.getByLabel('Monto inicial').fill('10000');
    const openResponse = page.waitForResponse(
      (res) => res.url().endsWith('/cash-sessions') && res.request().method() === 'POST',
    );
    await openDialog.getByRole('button', { name: 'Abrir caja' }).click();
    const openRes = await openResponse;
    expect(openRes.ok()).toBe(true);
    await expect(openDialog).toBeHidden({ timeout: 5_000 });

    // ─── Open session view ───
    await expect(page.getByText('Efectivo esperado')).toBeVisible();

    // ─── Egreso ───
    await page.getByRole('button', { name: 'Egreso' }).click();
    const expenseDialog = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Egreso' }) });
    await expect(expenseDialog).toBeVisible({ timeout: 5_000 });

    await expenseDialog.getByLabel('Monto').fill('2000');
    const movementResponse = page.waitForResponse(
      (res) => res.url().endsWith('/movements') && res.request().method() === 'POST',
    );
    await expenseDialog.getByRole('button', { name: 'Egreso' }).click();
    const movementRes = await movementResponse;
    expect(movementRes.ok()).toBe(true);
    await expect(expenseDialog).toBeHidden({ timeout: 5_000 });

    // Expected cash dropped by exactly the expense: 10000 - 2000 = 8000.
    // Unambiguous (unlike right after opening, when "Efectivo inicial" and
    // "Efectivo esperado" would show the same figure): the expense only
    // ever changes the second one.
    await expect(page.getByText('$8.000')).toBeVisible({ timeout: 5_000 });

    // ─── Cerrar caja ───
    await page.getByRole('button', { name: 'Cerrar caja' }).click();
    const closeDialog = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Cerrar caja' }) });
    await expect(closeDialog).toBeVisible({ timeout: 5_000 });

    await closeDialog.getByLabel('Efectivo contado').fill('7500');
    await expect(closeDialog.getByText(/Faltante/)).toBeVisible();

    const closeResponse = page.waitForResponse(
      (res) => res.url().endsWith('/close') && res.request().method() === 'POST',
    );
    await closeDialog.getByRole('button', { name: 'Cerrar caja' }).click();
    const closeRes = await closeResponse;
    expect(closeRes.ok()).toBe(true);

    // Closed result shown inline before returning to the closed state. Two
    // "Cerrar" buttons exist in the dialog at this point (the sr-only ✕ close
    // and the result view's own submit) — excluded by `data-slot`.
    await expect(closeDialog.getByText(/Faltante/)).toBeVisible({ timeout: 5_000 });
    await closeDialog.locator('button:not([data-slot="dialog-close"])', { hasText: 'Cerrar' }).click();
    await expect(closeDialog).toBeHidden({ timeout: 5_000 });

    // ─── Back to the closed state, with the session now in history ───
    await expect(page.getByText('La caja está cerrada')).toBeVisible();
    await expect(page.getByText(/Faltante/)).toBeVisible();
  });
});
