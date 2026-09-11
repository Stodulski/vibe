import { SkeletonPage } from '@/shared/components/common/Skeletons';

export function PageLoader() {
  return <SkeletonPage />;
}

// Public/auth chunks are small enough that a Suspense fallback would just
// be a flash between two loaders — the index.html skeleton already covers
// first paint, so render nothing while these resolve.
export function PublicPageLoader() {
  return null;
}
