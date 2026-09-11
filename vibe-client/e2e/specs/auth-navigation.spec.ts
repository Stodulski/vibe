import { test, expect } from '@playwright/test';

// Split out of `auth-validation.spec.ts` (slice 10, max-lines decomposition)
// — link navigation and protected-route redirects are a distinct concern
// from form validation, and splitting by feature area is the natural seam
// for Playwright specs (each file is its own `test.describe` unit).
test.describe('Auth Navigation', () => {
  test('login page has link back to register', async ({ page }) => {
    await page.goto('/login');

    const registerLink = page.getByRole('link', { name: /Registrate/ });
    await expect(registerLink).toBeVisible();
    await registerLink.click();

    await expect(page).toHaveURL(/register/);
  });

  test('register page has link back to login', async ({ page }) => {
    await page.goto('/register');

    const loginLink = page.getByRole('link', { name: /Inicia sesión/ });
    await expect(loginLink).toBeVisible();
    await loginLink.click();

    await expect(page).toHaveURL(/login/);
  });
});

test.describe('Protected Route Redirects', () => {
  test('protected route /courts redirects to login when unauthenticated', async ({ page }) => {
    await page.goto('/courts');
    await expect(page).toHaveURL(/login/);
  });

  test('protected route /clients redirects to login when unauthenticated', async ({ page }) => {
    await page.goto('/clients');
    await expect(page).toHaveURL(/login/);
  });

  test('protected route /settings redirects to login when unauthenticated', async ({ page }) => {
    await page.goto('/settings');
    await expect(page).toHaveURL(/login/);
  });

  test('protected route /bookings redirects to login when unauthenticated', async ({ page }) => {
    await page.goto('/bookings');
    await expect(page).toHaveURL(/login/);
  });
});
