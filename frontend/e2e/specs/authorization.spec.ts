import { test, expect } from '../helpers/auth.fixture';

// TST-06: no spec exercised what happens when an authenticated user's role
// doesn't allow the page it navigated to. `ProtectedRoute`
// (src/app/router/ProtectedRoute.tsx) renders `ForbiddenPage` in place — the
// route stays where it was, it's just not what the owner is allowed to see —
// rather than a silent redirect, which used to make a valid click look like
// it had done nothing.
test.describe('Role-based authorization', () => {
  test('an owner navigating to /admin sees the forbidden page, never the admin panel', async ({
    authenticatedPage: page,
  }) => {
    await page.goto('/admin');

    await expect(page).toHaveURL(/\/admin$/, { timeout: 10_000 });
    await expect(page.getByText('No tenés permiso para ver esta página')).toBeVisible({ timeout: 10_000 });
    await expect(page.getByRole('heading', { name: 'Usuarios' })).toHaveCount(0);
  });

  test('an owner navigating to a nested /admin route also sees the forbidden page', async ({
    authenticatedPage: page,
  }) => {
    await page.goto('/admin/users');

    await expect(page).toHaveURL(/\/admin\/users$/, { timeout: 10_000 });
    await expect(page.getByText('No tenés permiso para ver esta página')).toBeVisible({ timeout: 10_000 });
  });
});
