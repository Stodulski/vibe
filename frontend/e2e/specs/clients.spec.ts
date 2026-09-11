import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { createTestClients } from '../helpers/client-fixtures';

test.describe('Client Management', () => {
  let complexId: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
    await createTestClients(setup);
  });

  test('clients page loads', async ({ authenticatedPage: page }) => {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/clients');

    await expect(page.getByRole('heading', { name: 'Clientes', exact: true })).toBeVisible({
      timeout: 10_000,
    });
  });

  test('shows clients or empty state', async ({ authenticatedPage: page }) => {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/clients');

    await expect(page.getByRole('heading', { name: 'Clientes', exact: true })).toBeVisible({
      timeout: 10_000,
    });

    // Generous timeout: this shared complex accumulates bookings/clients
    // across every other authenticated spec in the same run (blocked-slots,
    // confirm-payment, owner-booking, ...), so the list has more to fetch
    // and render the later in the suite this test happens to run. `.first()`
    // because both ClienteUno AND ClienteDos are normally present at once
    // (createTestClients seeds both) -- this only needs "some real client or
    // the explicit empty state", not exactly one match.
    await expect(
      page
        .getByText('ClienteUno')
        .or(page.getByText('ClienteDos'))
        .or(page.getByText('Todavía no hay clientes registrados'))
        .first(),
    ).toBeVisible({ timeout: 20_000 });
  });
});
