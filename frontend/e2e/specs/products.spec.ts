import type { Page } from '@playwright/test';
import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { createApiHelper } from '../helpers/api.helper';

/**
 * The shared complex's catalog and cash session are as shared as everything
 * else on it (`cash.spec.ts`, `shared-setup.ts`, `TST-07`): every
 * authenticated spec runs serially against the same complex, and a rerun of
 * this spec (or a previous crashed run) can leave the till open and an old
 * "E2E Producto <timestamp>" product behind. The product name carries a
 * fresh `Date.now()` suffix (same convention as `owner-booking-create.spec.ts`)
 * so this run's row is never satisfied by a stale one. Every action below
 * happens on that product's own `/cash/products/:id` detail page — already
 * isolated from every other product in the shared catalog — rather than on
 * the list, where a page-wide match could hit an unrelated row.
 */
async function ensureTillOpen(page: Page): Promise<void> {
  await page.goto('/cash');
  await expect(page.getByRole('heading', { name: 'Caja', exact: true })).toBeVisible({ timeout: 10_000 });

  const isClosed = await page
    .getByText('La caja está cerrada')
    .isVisible()
    .catch(() => false);
  if (!isClosed) return;

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
}

/** Creates the run's own product from `/cash/products`. */
async function createProduct(page: Page, name: string): Promise<void> {
  await page.goto('/cash/products');
  await expect(page.getByRole('heading', { name: 'Productos', exact: true })).toBeVisible({ timeout: 10_000 });

  await page.getByRole('button', { name: 'Nuevo producto' }).first().click();
  const createDialog = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Nuevo producto' }) });
  await expect(createDialog).toBeVisible({ timeout: 5_000 });

  await createDialog.getByLabel('Nombre').fill(name);
  await createDialog.getByLabel('Precio').fill('1500');
  const createResponse = page.waitForResponse(
    (res) => res.url().endsWith('/products') && res.request().method() === 'POST',
  );
  await createDialog.getByRole('button', { name: 'Guardar' }).click();
  expect((await createResponse).ok()).toBe(true);
  await expect(createDialog).toBeHidden({ timeout: 5_000 });
}

/** Filters the list down to the run's own product (by its unique name) and opens its detail page. */
async function openProductDetail(page: Page, name: string): Promise<void> {
  await page.goto('/cash/products');
  await page.getByLabel('Buscar por nombre o categoría...').fill(name);
  const row = page.getByText(name, { exact: true });
  await expect(row).toBeVisible({ timeout: 5_000 });
  await row.click();
  await expect(page.getByRole('heading', { name: 'Detalle de producto', exact: true })).toBeVisible({
    timeout: 10_000,
  });
}

async function openRowAction(page: Page, name: string, action: string): Promise<void> {
  await page.getByRole('button', { name: `Más acciones: ${name}` }).click();
  await page.getByRole('menuitem', { name: action }).click();
}

/** Restocks 10 units for $5.000 (needs the open till) and checks the ledger row. */
async function restockTen(page: Page, name: string): Promise<void> {
  await openRowAction(page, name, 'Reponer');
  const restockDialog = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Reponer stock' }) });
  await expect(restockDialog).toBeVisible({ timeout: 5_000 });
  await restockDialog.getByLabel('Cantidad').fill('10');
  await restockDialog.getByLabel('Costo total').fill('5000');
  const restockResponse = page.waitForResponse(
    (res) => res.url().endsWith('/restock') && res.request().method() === 'POST',
  );
  await restockDialog.getByRole('button', { name: 'Reponer' }).click();
  expect((await restockResponse).ok()).toBe(true);
  await expect(restockDialog).toBeHidden({ timeout: 5_000 });

  const history = page.getByTestId('stock-movement-list');
  await expect(history.getByText('Reposición')).toBeVisible({ timeout: 5_000 });
  await expect(history.getByText('+10')).toBeVisible();
}

/** Counts 8 against a current stock of 10, so the adjustment is -2. */
async function adjustByCounting(page: Page, name: string): Promise<void> {
  const history = page.getByTestId('stock-movement-list');
  await openRowAction(page, name, 'Ajustar');
  const adjustDialog = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Ajustar stock' }) });
  await expect(adjustDialog).toBeVisible({ timeout: 5_000 });
  await expect(adjustDialog.getByTestId('adjust-current-stock')).toContainText('10');
  await adjustDialog.getByLabel('Cantidad contada').fill('8');
  await expect(adjustDialog.getByTestId('adjust-difference')).toContainText('-2');
  const adjustResponse = page.waitForResponse(
    (res) => res.url().endsWith('/adjustments') && res.request().method() === 'POST',
  );
  await adjustDialog.getByRole('button', { name: 'Ajustar' }).click();
  expect((await adjustResponse).ok()).toBe(true);
  await expect(adjustDialog).toBeHidden({ timeout: 5_000 });

  await expect(history.getByText('Ajuste')).toBeVisible({ timeout: 5_000 });
  await expect(history.getByText('-2')).toBeVisible();
}

async function deactivate(page: Page, name: string): Promise<void> {
  await openRowAction(page, name, 'Desactivar');
  const confirmDialog = page.getByRole('alertdialog').filter({ has: page.getByText('Desactivar producto') });
  await expect(confirmDialog).toBeVisible({ timeout: 5_000 });
  const deactivateResponse = page.waitForResponse(
    (res) => /\/products\/[^/]+$/.test(res.url()) && res.request().method() === 'PATCH',
  );
  await confirmDialog.getByRole('button', { name: 'Desactivar' }).click();
  expect((await deactivateResponse).ok()).toBe(true);
  await expect(confirmDialog).toBeHidden({ timeout: 5_000 });
}

test.describe('Products', () => {
  let complexId: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
    // Robust against a previous crashed run (or another spec file) having
    // left a session open — this spec always starts from a known state.
    await setup.api.closeAnyOpenCashSession(complexId);
  });

  test.afterAll(async () => {
    if (!complexId) return;
    const api = await createApiHelper();
    await api.closeAnyOpenCashSession(complexId);
  });

  test('owner can create a product, restock it, adjust it by counting, and deactivate it', async ({
    authenticatedPage: page,
  }) => {
    const name = `E2E Producto ${String(Date.now())}`;

    await createProduct(page, name);
    await ensureTillOpen(page);
    await openProductDetail(page, name);

    await restockTen(page, name);
    await adjustByCounting(page, name);
    await deactivate(page, name);

    await expect(page.getByText('Inactivo')).toBeVisible({ timeout: 5_000 });
  });
});
