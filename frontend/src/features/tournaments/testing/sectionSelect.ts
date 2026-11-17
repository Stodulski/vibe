import { ES_AR } from '@/shared/i18n/es_AR';
import { screen, userEvent } from '@/test/test-utils';

/** Opens the owner's section select of the category on screen and picks one of its sections. */
export async function pickSection(user: ReturnType<typeof userEvent.setup>, section: string) {
  await user.click(screen.getByRole('combobox', { name: ES_AR.tournaments.detail.sectionLabel }));
  await user.click(await screen.findByRole('option', { name: section }));
}
