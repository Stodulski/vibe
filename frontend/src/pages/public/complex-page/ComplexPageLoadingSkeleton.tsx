import { SkeletonComplexHeader, SkeletonSlotGrid } from '@/features/public-booking';

/** Mirrors the landing, which carries no step indicator — see ComplexPageContent. */
export function ComplexPageLoadingSkeleton() {
  return (
    <div className="animate-fade-in w-full space-y-6 sm:space-y-10">
      <SkeletonComplexHeader />
      <SkeletonSlotGrid />
    </div>
  );
}
