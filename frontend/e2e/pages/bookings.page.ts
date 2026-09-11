import { type Page } from '@playwright/test';

export class BookingsPage {
  constructor(private page: Page) {}

  get createButton() {
    return this.page.getByRole('button', { name: 'Nueva reserva' }).first();
  }

  get calendarTab() {
    return this.page.getByRole('tab', { name: 'Calendario' });
  }

  get listTab() {
    return this.page.getByRole('tab', { name: 'Lista' });
  }

  get prevDayButton() {
    return this.page.getByRole('button', { name: 'Día anterior' });
  }

  get nextDayButton() {
    return this.page.getByRole('button', { name: 'Día siguiente' });
  }

  async goto() {
    await this.page.goto('/bookings');
  }

  async createBooking(options: {
    courtName: string;
    time: string;
    clientFirstName: string;
    clientLastName: string;
    clientPhone: string;
  }) {
    await this.createButton.click();

    // Select court
    await this.page.getByRole('combobox', { name: 'Cancha' }).click();
    await this.page.getByRole('option', { name: options.courtName }).click();

    // Select time
    await this.page.getByRole('combobox', { name: 'Horario' }).click();
    await this.page.getByRole('option', { name: options.time }).click();

    // Fill client data
    await this.page.getByLabel('Nombre', { exact: true }).fill(options.clientFirstName);
    await this.page.getByLabel('Apellido').fill(options.clientLastName);
    await this.page.getByLabel('Teléfono').fill(options.clientPhone);

    // Submit
    await this.page.getByRole('button', { name: 'Nueva reserva' }).last().click();
  }
}
