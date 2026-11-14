import { InfoRow } from '@/shared/components/common/InfoRow';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { TournamentCategory } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

/** What the category is: its status, capacity, price per pair, format and how its matches are played. */
export function CategoryFacts({ category }: { category: TournamentCategory }) {
  return (
    <div className="space-y-3">
      {category.status === 'cancelled' && <p className="text-error-text text-sm">{t.detail.categoryCancelledNote}</p>}
      <InfoRow label={t.detail.capacity}>
        {t.pairsTaken(category.confirmed_count, category.capacity)} · {t.categoryStatus[category.status]}
      </InfoRow>
      <InfoRow label={t.detail.price}>
        <span className="score-text font-medium">{formatPrice(category.price)}</span>
      </InfoRow>
      <InfoRow label={t.detail.formatLabel}>
        <span className="block">{t.format[category.format]}</span>
        {category.format === 'groups' && (
          <span className="text-text-secondary block">
            {t.detail.groupsSetup(category.group_size, category.qualifiers_per_group)}
          </span>
        )}
        <span className="text-text-secondary block">
          {t.detail.matchSetup(category.sets_to_win, category.games_per_set, category.super_tiebreak_decider)}
        </span>
      </InfoRow>
    </div>
  );
}
