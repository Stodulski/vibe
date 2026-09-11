import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { createTestClients } from '../helpers/client-fixtures';

// Split out of `clients.spec.ts` (slice 10, max-lines decomposition) — the
// search-box scenarios are a distinct concern from the page-load/empty-state
// checks that remain in `clients.spec.ts`.
test.describe('Client Search', () => {
  let complexId: string;
  let hasClients = false;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
    hasClients = await createTestClients(setup);
  });

  test('search input is functional', async ({ authenticatedPage: page }) => {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/clients');

    await expect(page.getByRole('heading', { name: 'Clientes', exact: true })).toBeVisible({
      timeout: 10_000,
    });

    const searchInput = page.getByPlaceholder('Buscar...').first();
    await expect(searchInput).toBeVisible({ timeout: 5_000 });

    await searchInput.fill('TestSearch');
    await page.waitForTimeout(500);
    await expect(page.getByRole('heading', { name: 'Clientes', exact: true })).toBeVisible();

    await searchInput.clear();
    await page.waitForTimeout(500);
    await expect(page.getByRole('heading', { name: 'Clientes', exact: true })).toBeVisible();
  });

  test('search with no results shows appropriate state', async ({ authenticatedPage: page }) => {
    test.skip(!hasClients, 'No clients were created in beforeAll');
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/clients');

    await expect(page.getByRole('heading', { name: 'Clientes', exact: true })).toBeVisible({
      timeout: 10_000,
    });

    const searchInput = page.getByPlaceholder('Buscar...').first();
    await searchInput.fill('ZZZNoExiste999');
    await page.waitForTimeout(500);

    await expect(page.getByText('ClienteUno')).not.toBeVisible({ timeout: 5_000 });
  });
});
