import { expect, type Page } from '@playwright/test';

/**
 * Select the shared test complex and land on the (loaded) settings page.
 * Extracted from 7 duplicated call sites across `settings.spec.ts` and
 * `settings-tabs.spec.ts` (slice 10, max-lines decomposition — also removes
 * a DRY violation per this repo's own principles).
 */
export async function gotoSettings(page: Page, complexId: string): Promise<void> {
  await page.evaluate((id) => {
    localStorage.setItem('selectedComplexId', id);
  }, complexId);
  await page.goto('/settings');
  await expect(page.getByLabel('Nombre del complejo')).toBeVisible({ timeout: 10_000 });
}
