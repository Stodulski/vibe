import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/login.page';

test.describe('Auth Validation & Edge Cases', () => {
  test('login with empty fields shows validation errors', async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.goto();

    // Click submit without filling anything
    await loginPage.submitButton.click();

    // The form should not navigate away
    await expect(page).toHaveURL(/login/);
  });

  test('register step 1 with empty email shows validation error', async ({ page }) => {
    await page.goto('/register');

    // Step 1 only asks for email; clicking "Siguiente" without filling it
    // must not advance the wizard and must surface the real Zod message.
    await page.getByRole('button', { name: 'Siguiente' }).click();

    await expect(page.getByText('El email es requerido')).toBeVisible();
    await expect(page).toHaveURL(/register/);
    // Still on step 1 -- the personal-data fields aren't rendered yet.
    await expect(page.getByLabel('Nombre', { exact: true })).not.toBeVisible();
  });

  test('register with weak password shows error', async ({ page }) => {
    await page.goto('/register');

    await page.getByLabel('Email').fill('weak-password@test.com');
    await page.getByRole('button', { name: 'Siguiente' }).click();

    await page.getByLabel('Nombre', { exact: true }).fill('Test');
    await page.getByLabel('Apellido').fill('User');
    await page.getByLabel('Teléfono').fill('1112223333');
    await page.getByRole('button', { name: 'Siguiente' }).click();

    await page.locator('#password').fill('123');
    await page.locator('#confirm_password').fill('123');
    await page.getByRole('button', { name: 'Crear cuenta' }).click();

    // Validation prevents submission and shows the real message.
    await expect(page.getByText('Mínimo 8 caracteres')).toBeVisible();
    await expect(page).toHaveURL(/register/);
  });

  test('register with invalid email format stays on step 1', async ({ page }) => {
    await page.goto('/register');

    await page.getByLabel('Email').fill('not-an-email');
    await page.getByRole('button', { name: 'Siguiente' }).click();

    await expect(page.getByText('Email inválido')).toBeVisible();
    await expect(page).toHaveURL(/register/);
    await expect(page.getByLabel('Nombre', { exact: true })).not.toBeVisible();
  });

  test('forgot password with empty email shows validation', async ({ page }) => {
    await page.goto('/forgot-password');

    await page.getByRole('button', { name: /Enviar|Recuperar/ }).click();

    // Should remain on forgot-password page
    await expect(page).toHaveURL(/forgot-password/);
  });
});
