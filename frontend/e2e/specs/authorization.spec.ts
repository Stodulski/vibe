import { test, expect } from '../helpers/auth.fixture';

// TST-06: no spec exercised what happens when an authenticated user's role
// doesn't allow the page it navigated to. `ProtectedRoute` (src/app/router/ProtectedRoute.tsx)
// answers with a same-origin redirect rather than a 401/403 body: an owner
// denied `allowedRoles={['superadmin']}` gets `<Navigate to="/" replace />`,
// and `RootRedirect` then sends a non-superadmin to `/complexes` — there is
// no dedicated "forbidden" page to assert against, only where the owner
// actually lands.
test.describe('Role-based authorization', () => {
  test('an owner navigating to /admin is redirected away, never sees the admin panel', async ({
    authenticatedPage: page,
  }) => {
    await page.goto('/admin');

    await expect(page).toHaveURL(/\/complexes/, { timeout: 10_000 });
    await expect(page.getByRole('heading', { name: 'Usuarios' })).toHaveCount(0);
  });

  test('an owner navigating to a nested /admin route is also redirected away', async ({ authenticatedPage: page }) => {
    await page.goto('/admin/users');

    await expect(page).toHaveURL(/\/complexes/, { timeout: 10_000 });
  });
});
