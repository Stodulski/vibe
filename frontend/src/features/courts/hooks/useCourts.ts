// Moved to `shared/` so `features/complex` can read the same query; kept here
// so the courts feature and its tests keep importing it from where they did.
export { useCourts } from '@/shared/hooks/useCourts';
