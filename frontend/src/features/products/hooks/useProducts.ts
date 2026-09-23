// Moved to `shared/hooks/useProducts.ts` (pos-cashbox T5b): `features/cash`'s
// "Vender" screen needs the same active-only catalog query and features
// never import from one another. Re-exported here so existing imports from
// `../../hooks/useProducts` keep working.
export { useProducts } from '@/shared/hooks/useProducts';
