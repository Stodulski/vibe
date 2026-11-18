import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SOCIAL_KIT_FORMATS, SOCIAL_KIT_PIECES } from '@/shared/lib/publicPaths';
import { KitImageCardSkeleton } from '../components/KitImageCardSkeleton';
import { KIT_PIECE_GRID_CLASS } from '../components/kitLayout';

const t = ES_AR;

/** What `SocialKitPage` draws while its route chunk loads: the title, the intro, then each piece's two cards. */
export function SocialKitPageSkeleton() {
  return (
    <div className="space-y-8" role="status" aria-label={t.common.loading}>
      <PageHeaderSkeleton titleWidth="w-40" />
      <div className="max-w-xl space-y-2" aria-hidden="true">
        <Skeleton className="h-4 w-full rounded-lg" />
        <Skeleton className="h-4 w-2/3 rounded-lg" />
      </div>
      {SOCIAL_KIT_PIECES.map((piece) => (
        <section key={piece} className="space-y-4" aria-hidden="true">
          <Skeleton className="h-7 w-40 rounded-lg" />
          <div className={KIT_PIECE_GRID_CLASS}>
            {SOCIAL_KIT_FORMATS.map((format) => (
              <KitImageCardSkeleton key={format} format={format} />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
