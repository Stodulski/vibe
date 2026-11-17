import { ES_AR } from '@/shared/i18n/es_AR';
import { screen, userEvent } from '@/test/test-utils';

/** Opens the actions menu of the match at `index`, in the order the owner's view lists them. */
export async function openMatchMenu(user: ReturnType<typeof userEvent.setup>, index = 0) {
  const triggers = screen.getAllByRole('button', { name: ES_AR.tournaments.matches.actionsLabel });
  const trigger = triggers[index];
  if (!trigger) throw new Error(`the match ${String(index)} has no actions menu`);
  await user.click(trigger);
}

/** The labels of the items open in the match menu on screen, in order. */
export function menuLabels(): (string | null)[] {
  return screen.getAllByRole('menuitem').map((item) => item.textContent);
}
