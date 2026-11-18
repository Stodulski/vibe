import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SOCIAL_KIT_PIECES } from '@/shared/lib/publicPaths';
import { KitImageCardSkeleton } from '../components/KitImageCardSkeleton';
import { KIT_GRID_CLASS, KIT_SWITCH_CLASS } from '../components/kitLayout';

const t = ES_AR;

/** What `SocialKitPage` draws while its route chunk loads: the title, the intro, the switch and the two story cards. */
export function SocialKitPageSkeleton() {
  return (
    <div className="space-y-6" role="status" aria-label={t.common.loading}>
      <PageHeaderSkeleton titleWidth="w-40" />
      <div className="max-w-xl space-y-2" aria-hidden="true">
        <Skeleton className="h-4 w-full rounded-lg" />
        <Skeleton className="h-4 w-2/3 rounded-lg" />
      </div>
      <div className="space-y-2" aria-hidden="true">
        <div className={KIT_SWITCH_CLASS}>
          <Skeleton className="h-10 rounded-xl" />
          <Skeleton className="h-10 rounded-xl" />
        </div>
        <Skeleton className="h-4 w-64 max-w-full rounded-lg" />
      </div>
      <div className={KIT_GRID_CLASS} aria-hidden="true">
        {SOCIAL_KIT_PIECES.map((piece) => (
          <KitImageCardSkeleton key={piece} format="historia" />
        ))}
      </div>
    </div>
  );
}
