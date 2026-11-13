import { ES_AR } from '@/shared/i18n/es_AR';
import { ClientCardSkeleton } from './client-card/ClientCardSkeleton';
import { ClientTableSkeleton } from './ClientTableSkeleton';
import { CLIENT_GRID_CLASS } from './clientLayout';

const t = ES_AR;

/**
 * Loading shape of `ClientGrid` at each width: the card grid below `xl`, the
 * table from `xl` up. Both are in the tree and switched with CSS, which is
 * enough for a placeholder.
 */
export function ClientGridSkeleton() {
  return (
    <div role="status" aria-label={t.common.loading}>
      <div className={`${CLIENT_GRID_CLASS} xl:hidden`}>
        {Array.from({ length: 5 }, (_, i) => (
          <ClientCardSkeleton key={i} />
        ))}
      </div>
      <div className="hidden xl:block">
        <ClientTableSkeleton />
      </div>
    </div>
  );
}
