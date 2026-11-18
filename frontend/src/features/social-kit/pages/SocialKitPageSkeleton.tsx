import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SOCIAL_KIT_PIECES } from '@/shared/lib/publicPaths';
import { KitPieceRowSkeleton } from '../components/KitPieceRowSkeleton';
import { KIT_LIST_CLASS } from '../components/kitLayout';

const t = ES_AR;

/** What `SocialKitPage` draws while its route chunk loads: the title, the intro and one row per piece. */
export function SocialKitPageSkeleton() {
  return (
    <div className="space-y-6" role="status" aria-label={t.common.loading}>
      <PageHeaderSkeleton titleWidth="w-40" />
      <div className="max-w-xl space-y-2" aria-hidden="true">
        <Skeleton className="h-4 w-full rounded-lg" />
        <Skeleton className="h-4 w-2/3 rounded-lg" />
      </div>
      <div className={KIT_LIST_CLASS}>
        {SOCIAL_KIT_PIECES.map((piece) => (
          <KitPieceRowSkeleton key={piece} />
        ))}
      </div>
    </div>
  );
}
