import { z } from 'zod';

/** `GET complexes/{id}/instagram`. Never carries the token. */
export const instagramStatusSchema = z.object({
  /** False while the feature is off on the server: connecting is refused, and the panel hides the tab. */
  enabled: z.boolean(),
  connected: z.boolean(),
  /** A connection exists but its token is no longer usable; `username` is kept. */
  needs_reconnect: z.boolean(),
  username: z.string().nullable(),
  account_type: z.string().nullable(),
  expires_at: z.string().nullable(),
});

export type InstagramStatus = z.infer<typeof instagramStatusSchema>;

/**
 * `POST complexes/{id}/instagram/connect`. The URL is followed with a full
 * page navigation, so anything but https (a `javascript:` URL above all) is
 * refused here rather than handed to `window.location`.
 */
export const instagramConnectResponseSchema = z.object({
  authorize_url: z.url({ protocol: /^https$/ }),
});

export const instagramDisconnectResponseSchema = z.object({
  connected: z.boolean(),
});
