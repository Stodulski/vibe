import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';

test.describe('Dashboard', () => {
  let complexId: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
  });

  test('dashboard page loads and shows title', async ({ authenticatedPage: page }) => {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/dashboard');

    // "Dashboard" renders as two <h1>s: PageHeader's (mobile-only, md:hidden,
    // inside #main-content) and AppMasthead's desktop title (hidden below
    // md, lives in the fixed top bar outside #main-content) -- see
    // PageHeader.tsx / AppMasthead.tsx. Only one is actually visible at any
    // given viewport, so filter on :visible rather than assume either DOM
    // location.
    await expect(page.locator('h1:visible').filter({ hasText: 'Dashboard' })).toBeVisible({
      timeout: 10_000,
    });
  });

  // The four standalone stat cards (Reservas hoy / Ingreso hoy / Ocupación /
  // Pendientes) were folded into the "Control de caja" payment-overview panel
  // -- see PaymentOverview.tsx's comment. Assert what that panel actually
  // shows today.
  test('displays payment overview panel', async ({ authenticatedPage: page }) => {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/dashboard');

    await expect(page.getByText('Reservas hoy', { exact: true })).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText('Control de caja', { exact: true })).toBeVisible();
    await expect(page.getByText('Ocupación', { exact: true })).toBeVisible();
    await expect(page.getByText('Estado de cobro', { exact: true })).toBeVisible();
  });

  test('displays revenue section', async ({ authenticatedPage: page }) => {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/dashboard');

    await expect(page.getByText('Ingresos')).toBeVisible({ timeout: 15_000 });
  });

  // "Estado de canchas" (a live per-court status section) no longer exists on
  // the dashboard -- it was removed along with the standalone stat cards; see
  // DashboardContent.tsx, which now composes PaymentOverview + TodayBookings
  // plus a desktop-only "Tendencias" section (revenue chart, client insights,
  // occupancy heatmap). Assert that section instead of the removed one.
  test('shows the desktop trends section with the occupancy heatmap', async ({ authenticatedPage: page }) => {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/dashboard');

    await expect(page.getByText('Reservas hoy', { exact: true })).toBeVisible({ timeout: 15_000 });
    await expect(page.getByRole('heading', { name: 'Tendencias' })).toBeVisible({
      timeout: 10_000,
    });
    // DesktopHeatmap and MobileHeatmap both carry this exact aria-label at
    // once (one hidden via CSS per viewport) -- filter to whichever is
    // actually visible instead of assuming a single match.
    await expect(page.locator('[aria-label="Mapa de calor de ocupación por hora y día"]:visible')).toBeVisible({
      timeout: 10_000,
    });
  });
});
