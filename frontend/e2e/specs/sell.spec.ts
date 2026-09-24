import type { Page } from '@playwright/test';
import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { createApiHelper } from '../helpers/api.helper';
import type { ApiHelper, ApiProduct } from '../helpers/api.helper';

/**
 * Same shared-complex convention as `products.spec.ts`/`cash.spec.ts`: every
 * authenticated spec runs serially against one complex, so a rerun (or a
 * previous crashed run) can leave the till open. The product is created
 * through the API (its own uniquely-named row, never touching the shared
 * catalog's other products) so this spec exercises only "Vender" itself —
 * `products.spec.ts` already covers the "Productos" screen.
 */
async function ensureTillOpen(api: ApiHelper, complexId: string) {
  const current = await api.getCurrentCashSession(complexId);
  if (!current) await api.openCashSession(complexId, 10000);
}

/** Restocks the product to exactly 1 unit on hand — the sale below oversells it on purpose. */
async function createProductWithOneUnit(api: ApiHelper, complexId: string, name: string): Promise<ApiProduct> {
  const product = await api.createProduct(complexId, { name, price: 100000 });
  await api.restockProduct(complexId, product.id, 1, 10000);
  return product;
}

async function sellThreeUnits(page: Page, name: string) {
  await page.goto('/cash/sell');
  await expect(page.getByRole('heading', { name: 'Vender', exact: true })).toBeVisible({ timeout: 10_000 });

  await page.getByLabel('Buscar por nombre...').fill(name);
  // exact: the cart line's "Quitar/Restar/Sumar: <name>" buttons contain the
  // product name too, so a substring match turns ambiguous after the first tap.
  const tile = page.getByRole('button', { name, exact: true });
  await expect(tile).toBeVisible({ timeout: 5_000 });

  await tile.click();
  await tile.click();
  await tile.click();

  await expect(page.getByText('Sin stock suficiente: se vende igual')).toBeVisible({ timeout: 5_000 });
  await expect(page.getByTestId('sell-cart-total')).toHaveText('$3.000');

  const chargeResponse = page.waitForResponse(
    (res) => res.url().endsWith('/sales') && res.request().method() === 'POST',
  );
  await page.getByRole('button', { name: /^Cobrar \$3\.000$/ }).click();
  const response = await chargeResponse;
  expect(response.ok()).toBe(true);

  const confirmDialog = page
    .getByRole('dialog')
    .filter({ has: page.getByRole('heading', { name: 'Venta registrada' }) });
  await expect(confirmDialog).toBeVisible({ timeout: 5_000 });
  await expect(confirmDialog.getByText('$3.000')).toBeVisible();
  await expect(confirmDialog.getByText('Revisar stock')).toBeVisible();
  // 1 unit in stock, 3 sold: the warning must carry the resulting -2.
  await expect(confirmDialog.getByText(`${name}: -2`)).toBeVisible();
  await confirmDialog.getByRole('button', { name: 'Listo' }).click();
  await expect(confirmDialog).toBeHidden({ timeout: 5_000 });
}

/**
 * Regression guard for the products/sales delivery breaking Turno: the
 * backend writes a `sale` cash movement that the frontend's schema/types
 * used to reject entirely (a `CashMovement.category` enum missing `sale`
 * and `restock`), so the open session's own detail request failed to parse
 * and the whole Turno tab showed "No pudimos cargar la caja" instead of the
 * cash card — even though the sale itself had gone through.
 */
async function assertCashPageRendersSale(page: Page) {
  await page.goto('/cash');
  await expect(page.getByRole('heading', { name: 'Caja', exact: true })).toBeVisible({ timeout: 10_000 });
  await expect(page.getByText('No pudimos cargar la caja')).toHaveCount(0);
  await expect(page.getByText('Efectivo esperado')).toBeVisible({ timeout: 10_000 });
  await expect(page.getByText('Venta').first()).toBeVisible({ timeout: 5_000 });
}

async function voidTheSale(page: Page, name: string) {
  const salesList = page.getByTestId('sell-sales-list');
  // The row is found by its content, not by DOM depth or by its void
  // button (which disappears once the sale is voided).
  const row = salesList.getByTestId('sale-row').filter({ has: page.getByText(new RegExp(`3× ${name}`)) });
  await expect(row).toBeVisible({ timeout: 5_000 });

  await row.getByRole('button', { name: 'Anular' }).click();
  const voidDialog = page.getByRole('alertdialog').filter({ has: page.getByText('Anular venta') });
  await expect(voidDialog).toBeVisible({ timeout: 5_000 });

  const voidResponse = page.waitForResponse((res) => res.url().endsWith('/void') && res.request().method() === 'POST');
  await voidDialog.getByRole('button', { name: 'Anular' }).click();
  expect((await voidResponse).ok()).toBe(true);
  await expect(voidDialog).toBeHidden({ timeout: 5_000 });

  await expect(row.getByText('Anulada')).toBeVisible({ timeout: 5_000 });
  await expect(row.getByRole('button', { name: 'Anular' })).toHaveCount(0);
}

test.describe('Sell (POS)', () => {
  let complexId: string;
  let product: ApiProduct | undefined;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
    await setup.api.closeAnyOpenCashSession(complexId);
  });

  test.afterAll(async () => {
    if (!complexId) return;
    const api = await createApiHelper();
    await api.closeAnyOpenCashSession(complexId);
    if (product) await api.deactivateProduct(complexId, product.id);
  });

  test('owner can oversell a product, see the stock warning, and void the sale', async ({
    authenticatedPage: page,
  }) => {
    const name = `E2E Vender ${String(Date.now())}`;
    const api = await createApiHelper();

    await ensureTillOpen(api, complexId);
    product = await createProductWithOneUnit(api, complexId, name);

    await sellThreeUnits(page, name);
    await assertCashPageRendersSale(page);
    await page.goto('/cash/sell');
    await voidTheSale(page, name);
  });
});
