import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import type { Page } from '@playwright/test';

/**
 * "Dashboard"/"Reservas"/etc. render as two <h1>s at once: PageHeader's
 * (mobile-only, md:hidden, inside #main-content) and AppMasthead's desktop
 * title (hidden below md, in the fixed top bar outside #main-content) — see
 * PageHeader.tsx / AppMasthead.tsx. Only one is visible at any viewport.
 */
function visibleHeading(page: Page, text: string) {
  return page.locator('h1:visible').filter({ hasText: text });
}

/**
 * The desktop sidebar is permanently icon-only (SidebarNavLink.tsx: its
 * `<span>{item.label}</span>` only renders when NOT collapsed, and the icon
 * itself is `aria-hidden`), so its links carry no accessible name at this
 * viewport — the label only ever reaches a sighted user via a hover
 * tooltip. Locate by `href` instead of role+name.
 *
 * Extracted out of the test body to keep the describe callback under the
 * repo's max-lines-per-function cap.
 */
async function selectComplex(page: Page, complexId: string): Promise<void> {
  await page.evaluate((id) => {
    localStorage.setItem('selectedComplexId', id);
  }, complexId);
}

async function walkSidebarNav(page: Page): Promise<void> {
  await page.locator('a[href="/bookings"]').click();
  await expect(page).toHaveURL(/bookings/);
  await expect(visibleHeading(page, 'Reservas')).toBeVisible({ timeout: 10_000 });

  await page.locator('a[href="/courts"]').click();
  await expect(page).toHaveURL(/courts/);

  await page.locator('a[href="/clients"]').click();
  await expect(page).toHaveURL(/clients/);
  await expect(visibleHeading(page, 'Clientes')).toBeVisible({ timeout: 10_000 });

  await page.locator('a[href="/dashboard"]').click();
  await expect(page).toHaveURL(/dashboard/);
}

test.describe('Navigation Flows', () => {
  let complexId: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
  });

  test('sidebar navigation between pages', async ({ authenticatedPage: page }) => {
    await selectComplex(page, complexId);
    await page.goto('/dashboard');
    await expect(visibleHeading(page, 'Dashboard')).toBeVisible({ timeout: 10_000 });

    await walkSidebarNav(page);
  });

  test('deep link to dashboard loads correctly', async ({ authenticatedPage: page }) => {
    await selectComplex(page, complexId);
    await page.goto('/dashboard');

    await expect(visibleHeading(page, 'Dashboard')).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText('Reservas hoy', { exact: true })).toBeVisible({ timeout: 15_000 });
  });

  test('deep link to bookings loads correctly', async ({ authenticatedPage: page }) => {
    await selectComplex(page, complexId);
    await page.goto('/bookings');

    await expect(visibleHeading(page, 'Reservas')).toBeVisible({ timeout: 10_000 });
  });

  test('deep link to settings loads correctly', async ({ authenticatedPage: page }) => {
    await selectComplex(page, complexId);
    await page.goto('/settings');

    await expect(page.getByLabel('Nombre del complejo')).toBeVisible({ timeout: 10_000 });
  });

  test('browser back button works between pages', async ({ authenticatedPage: page }) => {
    await selectComplex(page, complexId);
    await page.goto('/dashboard');
    await expect(visibleHeading(page, 'Dashboard')).toBeVisible({ timeout: 10_000 });

    // Navigate to bookings (href-based -- see walkSidebarNav's comment above)
    await page.locator('a[href="/bookings"]').click();
    await expect(page).toHaveURL(/bookings/);

    // Go back
    await page.goBack();
    await expect(page).toHaveURL(/dashboard/);
  });

  test('page refresh preserves authenticated state', async ({ authenticatedPage: page }) => {
    await selectComplex(page, complexId);
    await page.goto('/dashboard');
    await expect(visibleHeading(page, 'Dashboard')).toBeVisible({ timeout: 10_000 });

    // Refresh the page
    await page.reload();

    // Should still be on dashboard (not redirected to login)
    await expect(visibleHeading(page, 'Dashboard')).toBeVisible({ timeout: 15_000 });
  });

  test('navigate to profile page', async ({ authenticatedPage: page }) => {
    await selectComplex(page, complexId);
    await page.goto('/profile');

    // Profile page should load
    await expect(visibleHeading(page, 'Mi perfil')).toBeVisible({ timeout: 10_000 });
  });
});
