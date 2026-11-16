import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@/test/test-utils';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import {
  button,
  field,
  fillTournament,
  next,
  pickDay,
  pickOption,
  reachReview,
  renderModal,
} from '../testing/createModalHelpers';

vi.mock('../api/tournaments.api', () => ({ tournamentsApi: { create: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.tournaments;
const ISO_DAY: unknown = expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/);

describe('CreateTournamentModal: steps', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('starts on the tournament step with the sport and the days, and no name to type', async () => {
    await renderModal();

    expect(screen.getByRole('listitem', { current: 'step' })).toHaveTextContent(t.createSteps.step1);
    expect(field(t.form.sport)).toBeInTheDocument();
    expect(screen.queryByLabelText(/Nombre del torneo/)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.form.addCategory })).not.toBeInTheDocument();
  });

  it('asks for both days before leaving step 1, and stays there', async () => {
    const { user } = await renderModal();

    await next(user);

    expect(await screen.findAllByText(ES_AR.validation.dateRequired)).toHaveLength(2);
    expect(screen.getByRole('listitem', { current: 'step' })).toHaveTextContent(t.createSteps.step1);
  });

  it('refuses a last day before the first one on step 1', async () => {
    const { user } = await renderModal();
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

    expect(field(t.form.sport)).toHaveTextContent('Pádel');
    expect(screen.queryByText(ES_AR.validation.dateRequired)).not.toBeInTheDocument();
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

  it('reviews the composed title and each category before creating', async () => {
    const { user } = await renderModal();
    await reachReview(user);

    expect(screen.getByRole('listitem', { current: 'step' })).toHaveTextContent(t.createSteps.step3);
    expect(screen.getByText('Torneo de pádel')).toBeInTheDocument();
    expect(screen.getByText('Caballeros 6ta')).toBeInTheDocument();
    expect(screen.getByText(t.format.groups, { exact: false })).toBeInTheDocument();
    expect(screen.getByText(t.detail.groupsSetup(3, 2), { exact: false })).toBeInTheDocument();
    expect(button(t.form.submit)).toBeInTheDocument();
  });

  it('sends the category with its modality, level, start and match format, and never a name', async () => {
    const { tournamentsApi, onClose, user } = await renderModal();
    await reachReview(user);

    await user.click(button(t.form.submit));

    await waitFor(() => {
      expect(tournamentsApi.create).toHaveBeenCalledTimes(1);
    });
    const body = vi.mocked(tournamentsApi.create).mock.calls[0]?.[1];
    expect(body).toMatchObject({ sport: 'padel', start_date: ISO_DAY, end_date: ISO_DAY });
    expect(body).not.toHaveProperty('name');
    expect(body?.categories).toEqual([
      {
        modality: 'men',
        level: 6,
        starts_on: ISO_DAY,
        starts_at: '18:00',
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

describe('CreateTournamentModal: suma', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('sends a suma with its prizes, and no level', async () => {
    const { tournamentsApi, user } = await renderModal();
    await fillTournament(user);
    await next(user);
    await user.click(button(t.form.addCategory));
    await user.click(button(t.traits.modality.mixed));
    await pickOption(user, t.form.kind, t.form.kindSuma);
    await user.type(field(t.form.suma), '15');
    await pickDay(user, t.form.startsOn, false);
    await pickOption(user, t.form.startsAt, '20:00');
    await user.type(field(t.form.price), '20000');
    await user.type(field(t.form.prizes), 'Trofeo para los campeones');
    await next(user);
    await user.click(button(t.createSteps.saveCategory));
    await next(user);

    await user.click(button(t.form.submit));

    await waitFor(() => {
      expect(tournamentsApi.create).toHaveBeenCalledTimes(1);
    });
    const category = vi.mocked(tournamentsApi.create).mock.calls[0]?.[1].categories[0];
    expect(category).toMatchObject({
      modality: 'mixed',
      suma: 15,
      starts_at: '20:00',
      prizes: 'Trofeo para los campeones',
    });
    expect(category).not.toHaveProperty('level');
    expect(category).not.toHaveProperty('name');
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
    await user.click(button(t.traits.modality.men));
    await pickOption(user, t.form.level, '6ta');
    await pickDay(user, t.form.startsOn, false);
    await pickOption(user, t.form.startsAt, '18:00');
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
        errors: [{ field: 'start_date', message: 'required' }],
      }),
    );
    await reachReview(user);

    await user.click(button(t.form.submit));

    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(screen.getByRole('listitem', { current: 'step' })).toHaveTextContent(t.createSteps.step1);
    expect(field(t.form.sport)).toBeInTheDocument();
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
    expect(screen.getByLabelText(`${t.form.sport} (${ES_AR.common.requiredMarker})`)).toBeInTheDocument();
    await fillTournament(user);
    await next(user);
    await user.click(button(t.form.addCategory));

    const input = screen.getByLabelText(`${t.form.courtFee} (${ES_AR.common.optional})`);
    const hint = screen.getByText(t.form.courtFeeHint);
    expect(input.compareDocumentPosition(hint) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(hint).not.toHaveTextContent(/^Opcional/);
  });
});
