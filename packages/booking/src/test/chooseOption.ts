import { screen } from '@testing-library/react';
import type { UserEvent } from '@testing-library/user-event';

/** Picks an option of a Radix Select: opens it by its trigger, then clicks the option by name. */
export async function chooseOption(user: UserEvent, trigger: HTMLElement, option: string | RegExp): Promise<void> {
  await user.click(trigger);
  await user.click(await screen.findByRole('option', { name: option }));
}
