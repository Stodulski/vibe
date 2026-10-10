import type { ReactNode } from 'react';
import type { BookingRootProps } from '../BookingRoot';
import type { BookingConfig } from '../config';

/**
 * Props shared by every booking entry. Each entry mounts its page inside its
 * own `BookingRoot`, so a host renders one component and gets the providers,
 * the config and the endpoints with it.
 */
export interface BookingEntryProps {
  slug: string;
  config: BookingConfig;
  /** Seeds the complex query. Only the complex page reads it. */
  initialComplex?: BookingRootProps['initialComplex'];
  /** Mount a Toaster. Pass `false` where the host app already mounts one. */
  toaster?: boolean;
  /** Rendered inside the providers, before the page: head tags and other host wiring. */
  children?: ReactNode;
}
