import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@/test/test-utils';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { button, field, fillTournament, next, pickDay, reachReview, renderModal } from '../testing/createModalHelpers';

vi.mock('../api/tournaments.api', () => ({ tournamentsApi: { create: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.tournaments;

describe('CreateTournamentModal: steps', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('starts on the tournament step with only its fields', async () => {
    await renderModal();

    expect(screen.getByRole('listitem', { current: 'step' })).toHaveTextContent(t.createSteps.step1);
    expect(field(t.form.name)).toBeInTheDocument();
    expect(field(t.form.sport)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.form.addCategory })).not.toBeInTheDocument();
  });

  it('asks for the name and both days before leaving step 1, and stays there', async () => {
    const { user } = await renderModal();

    await next(user);

    expect(await screen.findByText(t.validation.nameRequired)).toBeInTheDocument();
    expect(screen.getAllByText(ES_AR.validation.dateRequired)).toHaveLength(2);
    expect(screen.getByRole('listitem', { current: 'step' })).toHaveTextContent(t.createSteps.step1);
  });

  it('refuses a last day before the first one on step 1', async () => {
    const { user } = await renderModal();
    await user.type(field(t.form.name), 'Copa Vibe');
    await pickDay(user, t.form.startDate, true);
    await pickDay(user, t.form.endDate, false);

    await next(user);

    expect(await screen.findByText(t.validation.endBeforeStart)).toBeInTheDocument();
    expect(screen.getByRole('listitem', { current: 'step' })).toHaveTextContent(t.createSteps.step1);
  });

  it('goes to categories once step 1 is valid, and back keeps the values', async () => {
    const { user } = await renderModal();
    await fillTournament(user);

    await next(user);
    expect(screen.getByRole('listitem', { current: 'step' })).toHaveTextContent(t.createSteps.step2);
    await user.click(button(ES_AR.common.back));

    expect(field(t.form.name)).toHaveValue('  Copa Vibe ');
    expect(screen.queryByText(t.validation.nameRequired)).not.toBeInTheDocument();
  });

  it('asks for at least one category before leaving step 2', async () => {
    const { user } = await renderModal();
    await fillTournament(user);
    await next(user);

    await next(user);

    expect(await screen.findByText(t.validation.categoriesRequired)).toBeInTheDocument();
    expect(screen.getByRole('listitem', { current: 'step' })).toHaveTextContent(t.createSteps.step2);
  });
});

describe('CreateTournamentModal: review and submit', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('reviews the tournament and each category before creating', async () => {
    const { user } = await renderModal();
    await reachReview(user);

    expect(screen.getByRole('listitem', { current: 'step' })).toHaveTextContent(t.createSteps.step3);
    expect(screen.getByText('Copa Vibe')).toBeInTheDocument();
    expect(screen.getByText('6ta')).toBeInTheDocument();
    expect(screen.getByText(t.format.groups, { exact: false })).toBeInTheDocument();
    expect(screen.getByText(t.detail.groupsSetup(3, 2), { exact: false })).toBeInTheDocument();
    expect(button(t.form.submit)).toBeInTheDocument();
  });

  it('sends the category with its group and match format and the price in centavos', async () => {
    const { tournamentsApi, onClose, user } = await renderModal();
    await reachReview(user);

    await user.click(button(t.form.submit));

    await waitFor(() => {
      expect(tournamentsApi.create).toHaveBeenCalledTimes(1);
    });
    const body = vi.mocked(tournamentsApi.create).mock.calls[0]?.[1];
    expect(body).toMatchObject({ name: 'Copa Vibe', sport: 'padel' });
    expect(body?.categories).toEqual([
      {
        name: '6ta',
        format: 'groups',
        capacity: 16,
        price: 2000000,
        group_size: 3,
        qualifiers_per_group: 2,
        sets_to_win: 2,
        games_per_set: 6,
        super_tiebreak_decider: true,
        match_duration_minutes: 90,
      },
    ]);
    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
  });
});

describe('CreateTournamentModal: submit', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('sends the court fee per pair in centavos when the owner sets one', async () => {
    const { tournamentsApi, user } = await renderModal();
    await fillTournament(user);
    await next(user);
    await user.click(button(t.form.addCategory));
    await user.type(field(t.form.categoryName), '6ta');
    await user.type(field(t.form.price), '20000');
    await user.type(field(t.form.courtFee), '1500');
    await next(user);
    await user.click(button(t.createSteps.saveCategory));
    await next(user);

    await user.click(button(t.form.submit));

    await waitFor(() => {
      expect(tournamentsApi.create).toHaveBeenCalledTimes(1);
    });
    expect(vi.mocked(tournamentsApi.create).mock.calls[0]?.[1].categories[0]).toMatchObject({ court_fee: 150000 });
  });

  it('goes back to the step with the problem when the server names a tournament field', async () => {
    const { tournamentsApi, user } = await renderModal();
    vi.mocked(tournamentsApi.create).mockRejectedValue(
      await makeConsumedHttpError(422, {
        type: 'https://vibe.com.ar/problems/validation',
        title: 'validation',
        errors: [{ field: 'name', message: 'required' }],
      }),
    );
    await reachReview(user);

    await user.click(button(t.form.submit));

    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(screen.getByRole('listitem', { current: 'step' })).toHaveTextContent(t.createSteps.step1);
    expect(field(t.form.name)).toHaveValue('  Copa Vibe ');
  });

  it('stays on the review with the message when the server error names no field', async () => {
    const { tournamentsApi, user } = await renderModal();
    vi.mocked(tournamentsApi.create).mockRejectedValue(await makeConsumedHttpError(500, {}));
    await reachReview(user);

    await user.click(button(t.form.submit));

    expect(await screen.findByText(t.form.createError)).toBeInTheDocument();
    expect(screen.getByRole('listitem', { current: 'step' })).toHaveTextContent(t.createSteps.step3);
  });
});

describe('CreateTournamentModal: required and optional', () => {
  it('marks every field required or optional in its label, with the hint below the field', async () => {
    const { user } = await renderModal();
    expect(screen.getByLabelText(`${t.form.name} (${ES_AR.common.requiredMarker})`)).toBeInTheDocument();
    await fillTournament(user);
    await next(user);
    await user.click(button(t.form.addCategory));

    const input = screen.getByLabelText(`${t.form.courtFee} (${ES_AR.common.optional})`);
    const hint = screen.getByText(t.form.courtFeeHint);
    expect(input.compareDocumentPosition(hint) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(hint).not.toHaveTextContent(/^Opcional/);
  });
});
