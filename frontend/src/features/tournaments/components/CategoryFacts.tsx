import { InfoRow } from '@/shared/components/common/InfoRow';
import { ES_AR } from '@/shared/i18n/es_AR';
import { categoryStartLabel } from '@/shared/lib/traitLabels';
import { formatPrice } from '@/shared/lib/utils';
import type { CourtFeeSummary } from '../lib/courtFee';
import type { TournamentOwnerCategory } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

/**
 * What the category is, as label/value rows inside the category card: capacity
 * and status, price per pair, court fee and what is collected of it, format and
 * how its matches are played. One column on a phone, two from `sm`. Money is
 * proportional with tabular figures.
 */
export function CategoryFacts({
  category,
  courtFee,
}: {
  category: TournamentOwnerCategory;
  courtFee: CourtFeeSummary;
}) {
  const startLabel = categoryStartLabel(category.starts_on, category.starts_at);
  return (
    <div className="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2">
      <InfoRow label={t.detail.capacity}>
        {t.pairsTaken(category.confirmed_count, category.capacity)} · {t.categoryStatus[category.status]}
      </InfoRow>
      <InfoRow label={t.detail.price}>
        <span className="font-medium tabular-nums">{formatPrice(category.price)}</span>
      </InfoRow>
      {startLabel && <InfoRow label={t.detail.startsLabel}>{startLabel}</InfoRow>}
      {category.prizes && (
        <div className="sm:col-span-2">
          <InfoRow label={t.detail.prizesLabel}>{category.prizes}</InfoRow>
        </div>
      )}
      <InfoRow label={t.courtFee.factLabel}>
        {category.court_fee > 0 ? (
          <>
            <span className="font-medium tabular-nums">{formatPrice(category.court_fee)}</span>{' '}
            {t.courtFee.perPairSuffix}
          </>
        ) : (
          t.courtFee.none
        )}
      </InfoRow>
      {(courtFee.collected > 0 || courtFee.pending > 0) && (
        <InfoRow label={t.courtFee.summaryLabel}>
          <span className="tabular-nums">
            {t.courtFee.summary(formatPrice(courtFee.collected), formatPrice(courtFee.pending))}
          </span>
        </InfoRow>
      )}
      <div className="sm:col-span-2">
        <InfoRow label={t.detail.formatLabel}>
          {t.format[category.format]}
          {category.format === 'groups' && (
            <>
              {' · '}
              <span className="text-text-secondary">
                {t.detail.groupsSetup(category.group_size, category.qualifiers_per_group)}
              </span>
            </>
          )}
          <span className="text-text-secondary block">
            {t.detail.matchSetup(category.sets_to_win, category.games_per_set, category.super_tiebreak_decider)}
          </span>
        </InfoRow>
      </div>
    </div>
  );
}
