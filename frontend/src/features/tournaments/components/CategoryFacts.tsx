import { InfoRow } from '@/shared/components/common/InfoRow';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { CourtFeeSummary } from '../lib/courtFee';
import type { TournamentOwnerCategory } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

/**
 * What the category is, as a compact grid so the section tabs stay near the top:
 * capacity and status, price per pair, court fee and what is collected of it,
 * format and how its matches are played. Monospace only on the numbers.
 */
export function CategoryFacts({
  category,
  courtFee,
}: {
  category: TournamentOwnerCategory;
  courtFee: CourtFeeSummary;
}) {
  return (
    <div className="space-y-2">
      {category.status === 'cancelled' && <p className="text-error-text text-sm">{t.detail.categoryCancelledNote}</p>}
      <div className="grid grid-cols-2 gap-x-4 gap-y-2.5">
        <InfoRow label={t.detail.capacity}>
          {t.pairsTaken(category.confirmed_count, category.capacity)} · {t.categoryStatus[category.status]}
        </InfoRow>
        <InfoRow label={t.detail.price}>
          <span className="score-text font-medium">{formatPrice(category.price)}</span>
        </InfoRow>
        <InfoRow label={t.courtFee.factLabel}>
          {category.court_fee > 0 ? (
            <>
              <span className="score-text font-medium">{formatPrice(category.court_fee)}</span>{' '}
              {t.courtFee.perPairSuffix}
            </>
          ) : (
            t.courtFee.none
          )}
        </InfoRow>
        {(courtFee.collected > 0 || courtFee.pending > 0) && (
          <InfoRow label={t.courtFee.summaryLabel}>
            {t.courtFee.summary(formatPrice(courtFee.collected), formatPrice(courtFee.pending))}
          </InfoRow>
        )}
        <div className="col-span-2">
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
    </div>
  );
}
