import { Skeleton } from '@/shared/components/ui/skeleton';
import type { SocialKitFormat } from '@/shared/lib/publicPaths';
import { cn } from '@/shared/lib/utils';
import { KIT_CARD_CLASS, KIT_PREVIEW_ASPECT_CLASS } from './kitLayout';

/** Loading stand-in for `KitImageCard`: the same frame, with the label, preview and button as bars. */
export function KitImageCardSkeleton({ format }: { format: SocialKitFormat }) {
  return (
    <div className={KIT_CARD_CLASS} aria-hidden="true">
      <Skeleton className="h-5 w-16 rounded-lg" />
      <Skeleton className={cn('w-full rounded-lg', KIT_PREVIEW_ASPECT_CLASS[format])} />
      <Skeleton className="h-9 w-full rounded-full" />
    </div>
  );
}
