import { InfoRow } from '@/shared/components/common/InfoRow';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { pesosToCentavos } from '@/shared/lib/money';
import { formatPrice } from '@/shared/lib/utils';
import { tournamentTitle } from '@/shared/lib/traitLabels';
import { tournamentDays } from '../../lib/tournamentView';
import { categoryFormTitle, categorySummaryLine } from './categorySummary';
import type { CategoryFormValues, TournamentFormValues } from '../../schemas/tournamentForm.schema';

const t = ES_AR.tournaments;
const c = t.createSteps;

function CategoryReview({ category, sport }: { category: CategoryFormValues; sport: string }) {
  const groups = category.format === 'groups';
  return (
    <li className="space-y-0.5 py-2.5">
      <p className="text-text-primary text-sm font-medium">{categoryFormTitle(category, sport)}</p>
      <p className="text-text-secondary text-xs">{categorySummaryLine(category)}</p>
      <p className="text-text-tertiary text-xs">
        {groups && `${t.detail.groupsSetup(category.group_size, category.qualifiers_per_group)}. `}
        {t.detail.matchSetup(category.sets_to_win, category.games_per_set, category.super_tiebreak_decider)}.{' '}
        {c.matchDuration(category.match_duration_minutes)}
      </p>
      <p className="text-text-tertiary text-xs">
        {category.court_fee ? t.courtFee.line(formatPrice(pesosToCentavos(category.court_fee))) : t.courtFee.none}
      </p>
      {category.prizes.trim() && (
        <p className="text-text-tertiary text-xs">
          {t.detail.prizesLabel}: {category.prizes.trim()}
        </p>
      )}
    </li>
  );
}

interface Props {
  values: TournamentFormValues;
  error: string | undefined;
  pending: boolean;
  onBack: () => void;
}

/** Step 3: the tournament and every category as they will be created; the primary action sends them. */
export function ReviewStep({ values, error, pending, onBack }: Props) {
  return (
    <>
      <div className="space-y-4">
        <p className="text-text-tertiary text-sm">{c.reviewIntro}</p>
        <section className="space-y-2">
          <h3 className="text-text-primary text-sm font-semibold">{c.tournamentHeading}</h3>
          <div className="grid grid-cols-2 gap-x-4 gap-y-2.5">
            <div className="col-span-2">
              <InfoRow label={c.titleLabel}>{tournamentTitle(values.sport)}</InfoRow>
            </div>
            <InfoRow label={c.sportLabel}>
              {ES_AR.courts.sportTypes[values.sport as keyof typeof ES_AR.courts.sportTypes]}
            </InfoRow>
            <InfoRow label={c.daysLabel}>
              <span className="inline-block first-letter:uppercase">
                {tournamentDays(values.start_date, values.end_date)}
              </span>
            </InfoRow>
          </div>
        </section>
        <section className="space-y-1">
          <h3 className="text-text-primary text-sm font-semibold">{c.categoriesHeading}</h3>
          <ul className="border-border-subtle divide-border-subtle divide-y rounded-xl border px-3.5">
            {values.categories.map((category, index) => (
              <CategoryReview key={index} category={category} sport={values.sport} />
            ))}
          </ul>
        </section>
        {error && (
          <p role="alert" className="text-error-text text-sm">
            {error}
          </p>
        )}
      </div>
      <SectionFooter
        onCancel={onBack}
        cancelDisabled={pending}
        cancelLabel={ES_AR.common.back}
        submitLabel={t.form.submit}
        pending={pending}
      />
    </>
  );
}
