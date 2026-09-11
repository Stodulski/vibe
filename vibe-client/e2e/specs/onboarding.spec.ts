import { test, expect, type Page } from '@playwright/test';

const API = `${process.env.E2E_BASE_URL ?? 'http://localhost:5173'}/api/v1`;

const ONBOARDING_USER = {
  email: 'onboarding-e2e@test.com',
  password: 'TestPassword123!',
  firstName: 'Onboarding',
  lastName: 'Test',
  phone: '+5491100000044',
};

async function registerAndLogin(page: Page): Promise<void> {
  await page.request.post(`${API}/auth/register`, {
    data: {
      email: ONBOARDING_USER.email,
      password: ONBOARDING_USER.password,
      first_name: ONBOARDING_USER.firstName,
      last_name: ONBOARDING_USER.lastName,
      phone: ONBOARDING_USER.phone,
    },
  });

  await page.goto('/login');
  const submitBtn = page.getByRole('button', { name: 'Iniciar sesión' });
  await submitBtn.waitFor({ state: 'visible', timeout: 10_000 });

  await page.getByLabel('Email').fill(ONBOARDING_USER.email);
  await page.locator('#password').fill(ONBOARDING_USER.password);
  await submitBtn.click();

  await page.waitForURL(/\/(complexes|onboarding|dashboard)/, { timeout: 15_000 });
}

function isOnOnboarding(page: Page): boolean {
  return page.url().includes('/onboarding');
}

/**
 * Navigate to `/onboarding` if not already there, then skip the test when
 * the user still isn't on the onboarding flow (they already have a complex).
 * Extracted from 3 duplicated call sites (slice 10, max-lines
 * decomposition — also removes a DRY violation per this repo's own
 * principles).
 */
async function ensureOnOnboardingOrSkip(page: Page): Promise<void> {
  if (!isOnOnboarding(page)) {
    await page.goto('/onboarding');
    await page.waitForURL(/\/(onboarding|dashboard|complexes)/, { timeout: 10_000 });
  }

  if (!isOnOnboarding(page)) {
    test.skip(true, 'User already has complexes, redirected away from onboarding');
  }
}

test.describe('Onboarding Flow', () => {
  test('new user is redirected after login', async ({ page }) => {
    await registerAndLogin(page);

    const url = page.url();
    expect(url.includes('/complexes') || url.includes('/onboarding') || url.includes('/dashboard')).toBe(true);
  });

  test('step 1 form is accessible on onboarding', async ({ page }) => {
    await registerAndLogin(page);
    await ensureOnOnboardingOrSkip(page);

    // Verify the complex name field is accessible
    await expect(page.getByLabel('Nombre del complejo')).toBeVisible({ timeout: 10_000 });
  });

  test('step 1 has complex form fields', async ({ page }) => {
    await registerAndLogin(page);
    await ensureOnOnboardingOrSkip(page);

    // The address used to be split into Dirección/Ciudad/Provincia inputs;
    // it is now a single Google Places autocomplete combobox ("Dirección")
    // that derives city/province from the selected prediction. There is no
    // standalone "Ciudad" field anymore.
    await expect(page.getByLabel('Nombre del complejo')).toBeVisible({ timeout: 10_000 });
    const addressField = page.getByLabel('Dirección');
    await expect(addressField).toBeVisible();
    await expect(addressField).toHaveAttribute('role', 'combobox');
  });

  test('can fill step 1 identity fields', async ({ page }) => {
    await registerAndLogin(page);
    await ensureOnOnboardingOrSkip(page);

    const nameField = page.getByLabel('Nombre del complejo');
    await expect(nameField).toBeVisible({ timeout: 10_000 });

    await nameField.fill('Complejo Onboarding E2E');
    // The slug ("URL pública") input auto-derives from the name and then
    // collapses into a read-only preview line — it only stays an editable
    // input while the slug is still empty. Assert the derived preview instead
    // of the (by-then-hidden) input.
    await expect(page.getByText('complejo-onboarding-e2e')).toBeVisible();
    // Not exact: the accessible label is "Teléfono * requerido" (RequiredMark
    // appends a screen-reader-only "requerido" after the asterisk).
    await page.getByLabel('Teléfono').fill('1100000044');

    const addressField = page.getByLabel('Dirección');
    await addressField.fill('Av. Libertador 1234');
    await expect(addressField).toHaveValue('Av. Libertador 1234');
  });

  // Google Places has no API key configured for the isolated e2e API, so no
  // prediction ever comes back for an address query — there is nothing to
  // select, and step 1 cannot be submitted (latitude/longitude/city/province
  // are only ever set from `onSelect`, see AddressField.tsx). Covered by
  // 'can fill step 1 identity fields' above up to that point.
  test('can fill step 1 and proceed to step 2', () => {
    test.fixme(true, 'needs Google Places API key to select an address suggestion in e2e');
  });
});
