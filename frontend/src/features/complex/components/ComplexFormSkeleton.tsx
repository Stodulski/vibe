import { Skeleton } from '@/shared/components/ui/skeleton';

/** One labelled input: label line, then the control. */
function FieldSkeleton({ tall = false }: { tall?: boolean }) {
  return (
    <div className="space-y-2">
      <Skeleton className="h-4 w-28 rounded-lg" />
      <Skeleton className={tall ? 'h-24 w-full rounded-lg' : 'h-10 w-full rounded-lg'} />
    </div>
  );
}

/**
 * Loading shape of the General tab's form (name, slug, description, address,
 * contact, amenities, schedule). The floor is the measured loaded height:
 * ~1650px at 320 (single column) and a little less in the 39rem card, whose
 * own padding and border (50px) the `lg` floor leaves out.
 */
export function ComplexFormSkeleton() {
  return (
    <div className="flex min-h-[1650px] flex-col lg:min-h-[1450px]">
      <Skeleton className="mb-1 h-4 w-24 rounded-lg" />
      <Skeleton className="mb-6 h-3 w-48 rounded-lg" />
      <div className="space-y-6">
        <FieldSkeleton />
        <FieldSkeleton />
        <FieldSkeleton tall />
        <FieldSkeleton />
        <FieldSkeleton />
        <FieldSkeleton />
        <div className="flex flex-wrap gap-2">
          {Array.from({ length: 8 }, (_, i) => (
            <Skeleton key={i} className="h-9 w-24 rounded-full" />
          ))}
        </div>
        {Array.from({ length: 7 }, (_, i) => (
          <Skeleton key={i} className="h-12 w-full rounded-lg" />
        ))}
        <Skeleton className="h-10 w-32 rounded-lg" />
      </div>
    </div>
  );
}
