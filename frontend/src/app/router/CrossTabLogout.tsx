import { useCrossTabLogout } from '@/features/auth';

/**
 * Mounts `useCrossTabLogout` at the router root, once, above every route
 * group — the same reach `ScrollToTop` has for scroll restoration and the PWA
 * update trigger (see `router.tsx`), kept in its own component so a
 * scroll-only concern and a cross-tab session concern do not have to share
 * one file to get that reach. Renders nothing.
 *
 * It needs `useNavigate`, which is only available inside the router —
 * `App.tsx` (outside `RouterProvider`) is not — which is why this is mounted
 * as part of the root route's element rather than higher up the tree.
 */
export function CrossTabLogout() {
  useCrossTabLogout();
  return null;
}
