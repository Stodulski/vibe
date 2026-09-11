import { type Page } from '@playwright/test';

export class CourtsPage {
  constructor(private page: Page) {}

  get createButton() {
    return this.page.getByRole('button', { name: 'Nueva cancha' });
  }

  get emptyState() {
    return this.page.getByText('Todavía no hay canchas');
  }

  async goto() {
    await this.page.goto('/courts');
  }

  async createCourt(options: { name: string; sport?: string; courtType?: string }) {
    await this.createButton.click();

    // Fill name
    await this.page.getByLabel('Nombre').fill(options.name);

    // Select sport if specified
    if (options.sport) {
      await this.page.getByRole('combobox', { name: 'Deporte' }).click();
      await this.page.getByRole('option', { name: options.sport }).click();
    }

    // Select court type if specified
    if (options.courtType) {
      await this.page.getByRole('combobox', { name: 'Tipo de cancha' }).click();
      await this.page.getByRole('option', { name: options.courtType }).click();
    }

    // Submit
    await this.page.getByRole('button', { name: 'Crear' }).click();
  }
}
