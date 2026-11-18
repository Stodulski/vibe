import { Skeleton } from '@/shared/components/ui/skeleton';
import type { SocialKitFormat } from '@/shared/lib/publicPaths';
import { cn } from '@/shared/lib/utils';
import { KIT_CARD_CLASS, KIT_PREVIEW_ASPECT_CLASS } from './kitLayout';

/** Loading stand-in for `KitImageCard`: the same frame, with the preview, title and button as bars. */
export function KitImageCardSkeleton({ format }: { format: SocialKitFormat }) {
  return (
    <div className={KIT_CARD_CLASS} aria-hidden="true">
      <Skeleton className={cn('w-full rounded-xl', KIT_PREVIEW_ASPECT_CLASS[format])} />
      <Skeleton className="mx-1 h-5 w-24 rounded-lg" />
      <Skeleton className="h-9 w-full rounded-full" />
    </div>
  );
}
