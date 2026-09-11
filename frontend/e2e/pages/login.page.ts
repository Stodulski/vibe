import { type Page } from '@playwright/test';

export class LoginPage {
  constructor(private page: Page) {}

  get emailInput() {
    return this.page.getByLabel('Email');
  }

  get passwordInput() {
    return this.page.locator('#password');
  }

  get submitButton() {
    return this.page.getByRole('button', { name: 'Iniciar sesión' });
  }

  get registerLink() {
    return this.page.getByRole('link', { name: 'Registrate acá' });
  }

  get forgotPasswordLink() {
    return this.page.getByRole('link', { name: /Olvidaste tu contraseña/ });
  }

  async goto() {
    await this.page.goto('/login');
  }

  async login(email: string, password: string) {
    await this.emailInput.fill(email);
    await this.passwordInput.fill(password);
    await this.submitButton.click();
  }
}
