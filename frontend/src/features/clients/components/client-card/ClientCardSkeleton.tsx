import { Skeleton } from '@/shared/components/ui/skeleton';
import { CLIENT_CARD_CLASS, CLIENT_CARD_STATS_CLASS } from '../clientLayout';

/**
 * Loading shape of `ClientCard`: each bar sits in a box as tall as the text
 * line it stands for, so the card is ~134px like the loaded one.
 */
export function ClientCardSkeleton() {
  return (
    <div aria-hidden="true" className={CLIENT_CARD_CLASS}>
      <div className="flex h-5 items-center">
        <Skeleton className="h-4 w-36 rounded-lg" />
      </div>
      <div className="mt-0.5 flex h-4 items-center">
        <Skeleton className="h-3 w-24 rounded-lg" />
      </div>
      <div className={CLIENT_CARD_STATS_CLASS}>
        {[0, 1].map((i) => (
          <div key={i} className="min-w-0">
            <div className="flex h-3.5 items-center">
              <Skeleton className="h-2.5 w-14 rounded-lg" />
            </div>
            <div className="mt-1 flex h-7 items-center">
              <Skeleton className="h-5 w-12 rounded-lg" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
