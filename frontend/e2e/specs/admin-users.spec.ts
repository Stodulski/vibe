import { test, expect } from '../helpers/auth.fixture';
import { TURNSTILE_TEST_TOKEN } from '../helpers/test-data';

const API = `${process.env.E2E_BASE_URL ?? 'http://localhost:5173'}/api/v1`;

// TST-06: no spec exercised a superadmin actually doing anything in
// /admin/*. AdminComplexDetailPage (src/pages/admin/AdminComplexDetailPage.tsx)
// is read-only — there is no "edit complex" action in the UI for the audit's
// suggested "admin editing a complex" flow to cover — so this exercises the
// one real write action /admin/users actually has instead: toggling a
// user's active status (adminApi.toggleUserActive).
//
// Targets a throwaway user registered just for this test, never TEST_OWNER:
// TEST_OWNER's sessions (e2e/.auth/owner-*.json) are shared by every other
// `authenticated`-project spec running in parallel, and this API enforces
// `is_active` per-request, not only at login — deactivating that account
// mid-run would 401 every other spec still using one of its pooled sessions.
test.describe('Admin manages users', () => {
  test('deactivates and reactivates a user from their detail page', async ({ adminPage: page }) => {
    const email = `e2e-admin-toggle-${String(Date.now())}@test.com`;
    const registerRes = await page.request.post(`${API}/auth/register`, {
      data: {
        email,
        password: 'TestPassword123!',
        first_name: 'Toggle',
        last_name: 'Target',
        phone: '+5491100000077',
        turnstile_token: TURNSTILE_TEST_TOKEN,
      },
    });
    expect(registerRes.status()).toBe(201);

    await page.goto('/admin/users');
    await expect(page.getByRole('heading', { name: 'Usuarios' })).toBeVisible({ timeout: 10_000 });

    await page.getByPlaceholder('Buscar por nombre o email...').fill(email);
    // Not `getByText(email)`: DesktopUsersTable's email cell
    // (src/features/admin/components/users-table/DesktopUsersTable.tsx) is
    // plain text with no click handler — only the name cell is a `<Link>` to
    // the detail page. `getByRole('row', ...)` also sidesteps
    // UsersTableResults rendering both the `md:hidden` mobile card list and
    // the desktop table unconditionally (only one visible per viewport):
    // MobileUserCard has no `role="row"`, so this can only match the real
    // `<table>` row.
    await page
      .getByRole('row', { name: /Toggle Target/ })
      .getByRole('link')
      .click();

    await expect(page).toHaveURL(/\/admin\/users\/[^/]+$/, { timeout: 10_000 });
    const deactivateButton = page.getByRole('button', { name: 'Desactivar usuario' });
    await expect(deactivateButton).toBeVisible({ timeout: 10_000 });

    await deactivateButton.click();
    await page.getByRole('alertdialog').getByRole('button', { name: 'Confirmar' }).click();
    await expect(page.getByText('Usuario desactivado')).toBeVisible({ timeout: 10_000 });

    const activateButton = page.getByRole('button', { name: 'Activar usuario' });
    await expect(activateButton).toBeVisible({ timeout: 10_000 });
    await activateButton.click();
    await page.getByRole('alertdialog').getByRole('button', { name: 'Confirmar' }).click();
    await expect(page.getByText('Usuario activado')).toBeVisible({ timeout: 10_000 });
  });
});
