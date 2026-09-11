import { type Page } from '@playwright/test';

export class PublicBookingPage {
  constructor(private page: Page) {}

  get complexName() {
    return this.page.locator('h1').first();
  }

  get continueButton() {
    return this.page.getByRole('button', { name: 'Continuar' });
  }

  get noAvailabilityMessage() {
    return this.page.getByText('No hay horarios disponibles');
  }

  get closedMessage() {
    return this.page.getByText('Cerrado');
  }

  async goto(slug: string) {
    await this.page.goto(`/${slug}`);
  }

  async selectFirstAvailableSlot() {
    // Click the first available slot (gridcell that is not disabled)
    const slot = this.page
      .getByRole('gridcell')
      .filter({ has: this.page.locator(':not([disabled])') })
      .first();
    await slot.click();
    return slot;
  }

  async fillBookingForm(options: { firstName: string; lastName: string; phone: string; email?: string }) {
    await this.page.getByLabel('Nombre', { exact: true }).fill(options.firstName);
    await this.page.getByLabel('Apellido').fill(options.lastName);
    await this.page.getByLabel('Teléfono').fill(options.phone);
    if (options.email) {
      await this.page.getByLabel('Email').fill(options.email);
    }
  }

  async submitBooking() {
    // The submit button text varies depending on payment mode
    await this.page.getByRole('button', { name: /Confirmar reserva|Pagar/ }).click();
  }
}
