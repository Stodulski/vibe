import { describe, it, expect, vi } from 'vitest';
import { screen } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { fillTournament, next, renderModal } from '../testing/createModalHelpers';

vi.mock('../api/tournaments.api', () => ({ tournamentsApi: { create: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

describe('CreateTournamentModal: categories step copy', () => {
  it('says what a category holds once, not in the header and again in the section', async () => {
    const { user } = await renderModal();
    await fillTournament(user);
    await next(user);

    expect(screen.getAllByText(ES_AR.tournaments.form.description)).toHaveLength(1);
  });
});
