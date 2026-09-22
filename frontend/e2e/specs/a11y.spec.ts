import AxeBuilder from '@axe-core/playwright';
import type { Page } from '@playwright/test';
import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';

/**
 * Runs axe against the current page and fails on any `serious` or `critical`
 * violation. `moderate`/`minor` findings are logged (via the returned
 * results, inspectable from a failed run's trace) but don't fail the test —
 * TST-11 asks for a floor, not zero-tolerance on every axe rule, which for a
 * page this size would drown real regressions in cosmetic contrast nits.
 */
async function expectNoSeriousViolations(page: Page): Promise<void> {
  const results = await new AxeBuilder({ page }).analyze();
  const severe = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical');
  const summary = severe.map((v) => `${v.id} (${String(v.impact)}): ${v.help} — ${String(v.nodes.length)} node(s)`);
  expect(severe, `serious/critical axe violations:\n${summary.join('\n')}`).toEqual([]);
}

test.describe('Accessibility — serious/critical axe violations', () => {
  let complexSlug: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexSlug = setup.complexSlug;
  });

  test('/login has no serious/critical violations', async ({ page }) => {
    await page.goto('/login');
    await expect(page.getByRole('button', { name: 'Iniciar sesión' })).toBeVisible({ timeout: 10_000 });
    await expectNoSeriousViolations(page);
  });

  test('/dashboard has no serious/critical violations', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard');
    await expect(page.locator('h1:visible').first()).toBeVisible({ timeout: 10_000 });
    await expectNoSeriousViolations(page);
  });

  test('/bookings has no serious/critical violations', async ({ authenticatedPage: page }) => {
    await page.goto('/bookings');
    await expect(page.getByRole('heading', { name: 'Reservas', exact: true })).toBeVisible({ timeout: 10_000 });
    await expectNoSeriousViolations(page);
  });

  test('/settings has no serious/critical violations', async ({ authenticatedPage: page }) => {
    await page.goto('/settings');
    await expect(page.getByLabel('Nombre del complejo')).toBeVisible({ timeout: 10_000 });
    await expectNoSeriousViolations(page);
  });

  test('the public complex page has no serious/critical violations', async ({ page }) => {
    await page.goto(`/${complexSlug}`);
    await expect(page.getByRole('heading', { level: 1 }).first()).toBeVisible({ timeout: 15_000 });
    await expectNoSeriousViolations(page);
  });
});
