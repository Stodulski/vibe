import { Skeleton } from '@/shared/components/ui/skeleton';

export function BookingLoadingSkeleton() {
  return (
    <div className="border-border-subtle bg-bg-subtle rounded-2xl border p-3 sm:p-4">
      <div className="space-y-2">
        {Array.from({ length: 5 }, (_, i) => (
          <Skeleton key={i} className="h-16 w-full rounded-xl" />
        ))}
      </div>
    </div>
  );
}
