import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';

// Split out of `court-management.spec.ts` (slice 10, max-lines
// decomposition) — read/edit/display checks on existing court cards are a
// distinct concern from the court-creation flow that remains in
// `court-management.spec.ts`.
test.describe('Court Management — Display & Edit', () => {
  let complexId: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
  });

  test('can edit a court name', async ({ authenticatedPage: page }) => {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/courts');

    // Wait for page to load and verify courts exist
    await expect(page.getByRole('button', { name: 'Nueva cancha' })).toBeVisible({
      timeout: 10_000,
    });

    // At least one court heading should be visible
    const courtHeading = page.getByRole('heading', { name: /Cancha/ }).first();
    await expect(courtHeading).toBeVisible({ timeout: 5_000 });

    // Find and click the edit button on the first court card
    const editButton = page.getByRole('button', { name: 'Editar' }).first();
    await expect(editButton).toBeVisible({ timeout: 5_000 });
    await editButton.click();

    // The edit dialog should open with the court form
    await expect(page.getByText('Editar cancha')).toBeVisible({ timeout: 5_000 });

    // Verify the name input is present and editable
    const nameInput = page.getByLabel('Nombre');
    await expect(nameInput).toBeVisible();
    await expect(nameInput).toHaveValue(/.+/);
  });

  test('court cards show sport and type info', async ({ authenticatedPage: page }) => {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/courts');

    // Wait for page to load and courts to appear
    await expect(page.getByRole('button', { name: 'Nueva cancha' })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByRole('heading', { name: /Cancha/ }).first()).toBeVisible({
      timeout: 5_000,
    });

    // Verify at least one court card displays sport type information
    // Sport types: Padel, Tenis, Futbol, Basquet
    await expect(page.getByText(/Pádel|Tenis|Fútbol|Básquet/).first()).toBeVisible({
      timeout: 5_000,
    });

    // Verify at least one court card displays court type information
    // Court types: Techada, Descubierta, Semi cubierta
    await expect(page.getByText(/Techada|Descubierta|Semi cubierta/).first()).toBeVisible({
      timeout: 5_000,
    });
  });

  test('court cards have delete button', async ({ authenticatedPage: page }) => {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/courts');

    await expect(page.getByRole('button', { name: 'Nueva cancha' })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByRole('heading', { name: /Cancha/ }).first()).toBeVisible({
      timeout: 5_000,
    });

    // Verify delete buttons exist on court cards
    await expect(page.getByRole('button', { name: 'Eliminar' }).first()).toBeVisible({
      timeout: 5_000,
    });
  });
});
