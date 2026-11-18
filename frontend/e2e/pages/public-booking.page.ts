import { expect, type Page } from '@playwright/test';
import { landingComplexUrl } from '../helpers/landing';

export class PublicBookingPage {
  constructor(private page: Page) {}

  get complexName() {
    return this.page.locator('h1').first();
  }

  /**
   * After an hour, the free courts are listed and tapping one continues to
   * the confirm page; there is no Continue button. The tap first holds the
   * slot (`POST /book/holds`, with the invisible Turnstile token when the
   * storefront has a site key), so this waits for that answer.
   */
  /**
   * Answers "¿Cuánto tiempo?" with the duration the bar opens on (1 h). The
   * hours only appear once it is answered.
   */
  async answerDuration() {
    await this.page
      .getByRole('region', { name: '¿Cuánto tiempo?' })
      .getByRole('button', { name: /^Elegir/ })
      .click();
    await expect(this.page.getByText('¿Cuánto tiempo?')).not.toBeVisible({ timeout: 10_000 });
  }

  async chooseFirstCourt() {
    const hold = this.page.waitForResponse(
      (res) => res.request().method() === 'POST' && new URL(res.url()).pathname.endsWith('/book/holds'),
      { timeout: 30_000 },
    );
    await this.page.getByRole('group', { name: 'Elegí la cancha' }).getByRole('button').first().click();
    return hold;
  }

  /**
   * Gives the held slot back from the confirm page ("Cambiar horario", which
   * sends `POST /book/holds/{token}/release` as a beacon). A spec that stops
   * on the confirm page calls this: the API caps live unconverted holds per
   * client address (3), and every spec shares one address, so a hold left
   * behind refuses a later spec's hold with 429 until it expires.
   */
  async releaseHold() {
    const release = this.page.waitForRequest(
      (req) => req.method() === 'POST' && /\/book\/holds\/[^/]+\/release$/.test(new URL(req.url()).pathname),
    );
    await this.page.getByRole('button', { name: 'Cambiar horario' }).click();
    return release;
  }

  /** The hold countdown of the confirm page: "mm:ss" on screen, named "Te guardamos el turno por". */
  get holdCountdown() {
    return this.page.getByRole('timer');
  }

  get noAvailabilityMessage() {
    return this.page.getByText('No hay horarios disponibles');
  }

  get closedMessage() {
    return this.page.getByText('Cerrado');
  }

  async goto(slug: string) {
    await this.page.goto(landingComplexUrl(slug));
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
    // The accessible names carry a suffix: "Nombre (requerido)", "Email (opcional)".
    await this.page.getByLabel(/^Nombre/).fill(options.firstName);
    await this.page.getByLabel(/^Apellido/).fill(options.lastName);
    await this.page.getByLabel(/^Teléfono/).fill(options.phone);
    if (options.email) {
      await this.page.getByLabel(/^Email/).fill(options.email);
    }
  }

  async submitBooking() {
    // The submit button text varies depending on payment mode
    await this.page.getByRole('button', { name: /Confirmar reserva|Pagar/ }).click();
  }
}
