import { TURNSTILE_TEST_TOKEN } from '../helpers/test-data';
import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/login.page';

const API = `${process.env.E2E_BASE_URL ?? 'http://localhost:5173'}/api/v1`;

test.describe('Auth Flow', () => {
  test('register with valid data shows success', async ({ page }) => {
    await page.goto('/register');

    // Register is now a 3-step wizard: Email -> Datos personales -> Contraseña.
    // Step 1: email.
    await page.getByLabel('Email').fill('pedro-e2e@test.com');
    await page.getByRole('button', { name: 'Siguiente' }).click();

    // Step 2: personal data.
    await page.getByLabel('Nombre', { exact: true }).fill('Pedro');
    await page.getByLabel('Apellido').fill('Martínez');
    await page.getByLabel('Teléfono').fill('1198765432');
    await page.getByRole('button', { name: 'Siguiente' }).click();

    // Step 3: password.
    await page.locator('#password').fill('TestPassword123!');
    await page.locator('#confirm_password').fill('TestPassword123!');

    // When VITE_TURNSTILE_SITE_KEY is set to Cloudflare's always-pass test
    // key (1x00000000000000000000AA), the widget auto-solves and the
    // button becomes enabled shortly after — this also covers the
    // no-Turnstile-configured case, where the button is already enabled.
    const submitButton = page.getByRole('button', { name: 'Crear cuenta' });
    await expect(submitButton).toBeEnabled({ timeout: 10_000 });
    await submitButton.click();

    // Should redirect to verify email sent page
    await expect(page).toHaveURL(/verify-email-sent/, { timeout: 10_000 });
  });

  test('login with invalid credentials shows error', async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login('invalid@test.com', 'wrongpassword');

    await expect(page.getByText('Email o contraseña incorrectos')).toBeVisible();
  });

  test('login with valid credentials redirects to complexes', async ({ page }) => {
    // First register a user
    await page.request.post(`${API}/auth/register`, {
      data: {
        email: 'auth-test@test.com',
        password: 'TestPassword123!',
        first_name: 'Auth',
        last_name: 'Test',
        phone: '+5491100000001',
        turnstile_token: TURNSTILE_TEST_TOKEN,
      },
    });

    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login('auth-test@test.com', 'TestPassword123!');

    await expect(page).toHaveURL(/complexes/, { timeout: 10_000 });
  });

  test('unauthenticated access to /dashboard redirects to /login', async ({ page }) => {
    await page.goto('/dashboard');

    await expect(page).toHaveURL(/login/);
  });

  test('forgot password link navigates correctly', async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.forgotPasswordLink.click();

    await expect(page).toHaveURL(/forgot-password/);
    await expect(page.getByText('Recuperar contraseña')).toBeVisible();
  });

  test('register link from login navigates correctly', async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.registerLink.click();

    await expect(page).toHaveURL(/register/);
  });
});
