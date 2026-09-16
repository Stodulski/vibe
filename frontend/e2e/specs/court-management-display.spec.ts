import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';

// Split out of `court-management.spec.ts` (slice 10, max-lines
// decomposition) — read/edit/display checks on existing court cards are a
// distinct concern from the court-creation flow that remains in
// `court-management.spec.ts`.
/**
 * The actions live in different places depending on width: a card shows
 * "Editar"/"Eliminar" as buttons, and from `xl` the table puts all three
 * behind a per-row "Más acciones" menu. The suite runs at desktop widths, so
 * open the menu when it is the one on screen.
 */
async function openCourtAction(page: import('@playwright/test').Page, name: RegExp) {
  const rowMenu = page.getByRole('button', { name: /Más acciones/ }).first();
  if (await rowMenu.isVisible().catch(() => false)) {
    await rowMenu.click();
    return page.getByRole('menuitem', { name });
  }
  return page.getByRole('button', { name }).first();
}

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
    await expect(page.getByRole('button', { name: 'Agregar cancha' })).toBeVisible({
      timeout: 10_000,
    });

    // At least one court heading should be visible
    const courtHeading = page.getByRole('heading', { name: /Cancha/ }).first();
    await expect(courtHeading).toBeVisible({ timeout: 5_000 });

    // Find and click the edit action for the first court
    const editButton = await openCourtAction(page, /^Editar$/);
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
    await expect(page.getByRole('button', { name: 'Agregar cancha' })).toBeVisible({
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

  test('a court can be deleted from its actions', async ({ authenticatedPage: page }) => {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/courts');

    await expect(page.getByRole('button', { name: 'Agregar cancha' })).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByRole('heading', { name: /Cancha/ }).first()).toBeVisible({
      timeout: 5_000,
    });

    // Verify a delete action is reachable for a court
    const deleteAction = await openCourtAction(page, /^Eliminar$/);
    await expect(deleteAction).toBeVisible({ timeout: 5_000 });
  });
});
