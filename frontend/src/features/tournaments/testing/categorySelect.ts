import { ES_AR } from '@/shared/i18n/es_AR';
import { screen, userEvent } from '@/test/test-utils';

/** Opens the owner's category select of the tournament on screen and picks the category with this title. */
export async function pickCategory(user: ReturnType<typeof userEvent.setup>, title: string) {
  await user.click(screen.getByRole('combobox', { name: ES_AR.tournaments.detail.categoriesLabel }));
  await user.click(await screen.findByRole('option', { name: title }));
}
