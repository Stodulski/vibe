// Moved to `shared/lib/today.ts` so other features read the same "today"; kept
// here so the bookings call sites and this folder's test do not change.
export { nowInArgentina, todayInArgentina } from '@/shared/lib/today';
