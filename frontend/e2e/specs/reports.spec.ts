import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';

test.describe('Reports', () => {
  let complexId: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
  });

  test('reports page loads', async ({ authenticatedPage: page }) => {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/reports');

    await expect(page.locator('h1:visible').filter({ hasText: 'Reportes' })).toBeVisible({
      timeout: 10_000,
    });
  });

  test('displays month and year selectors', async ({ authenticatedPage: page }) => {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/reports');

    await expect(page.locator('h1:visible').filter({ hasText: 'Reportes' })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.locator('#report-month')).toBeVisible({ timeout: 10_000 });
    await expect(page.locator('#report-year')).toBeVisible({ timeout: 10_000 });
  });

  test('shows export button', async ({ authenticatedPage: page }) => {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/reports');

    await expect(page.getByRole('button', { name: /Descargar Excel/ })).toBeVisible({
      timeout: 10_000,
    });
  });

  test('shows report content or error state', async ({ authenticatedPage: page }) => {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/reports');

    await expect(page.getByText('Reporte mensual')).toBeVisible({ timeout: 10_000 });

    // The empty state renders both a "Sin datos" heading AND a "No hay pagos
    // registrados..." description together, so this combined locator can
    // legitimately resolve to more than one element -- `.first()` asserts
    // "some recognized state rendered", not "exactly one text node did".
    await expect(
      page
        .getByText('Método')
        .or(page.getByText('Sin datos'))
        .or(page.getByText('Error al cargar el reporte'))
        .or(page.getByText('No hay pagos registrados'))
        .first(),
    ).toBeVisible({ timeout: 10_000 });
  });
});
