import { ExternalLink, Unplug, Loader2 } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function ConnectAction({
  connected,
  authUrl,
  onConnectClick,
  onDisconnectClick,
}: {
  connected: boolean;
  authUrl: string | null;
  onConnectClick: () => void;
  onDisconnectClick: () => void;
}) {
  return (
    // Left, like the save above it — two actions on one screen sitting at
    // opposite edges makes the reader look for a rule that is not there.
    //
    // And no rule of its own: the section already opens with one, so a second
    // line four elements later marks a boundary that was already marked.
    <SectionFooter align="start" className="border-t-0 pt-2">
      {connected ? (
        <Button
          variant="outline"
          size="sm"
          className="w-full border-error-border/30 text-error-text hover:bg-error-bg sm:w-auto"
          onClick={onDisconnectClick}
        >
          <Unplug className="size-3.5" />
          {t.mp.disconnect}
        </Button>
      ) : authUrl ? (
        // Secondary, not primary. This block now shares a screen with the
        // deposit form's "Guardar", and two solid brand-filled buttons on one
        // screen compete instead of ranking. "Guardar" is the recurring action
        // and keeps the fill; connecting is one-time setup, and the block
        // already announces itself with a status row and a green badge.
        <Button asChild variant="outline" className="w-full sm:w-auto">
          <a href={authUrl} onClick={onConnectClick}>
            <ExternalLink className="size-3.5" />
            {t.mp.connect}
          </a>
        </Button>
      ) : (
        // The auth URL builds asynchronously (PKCE challenge, and now the
        // app id from GET mp/status) — rendering nothing here for that window
        // dropped the button from the layout and put it back once the URL
        // was ready, which read as the page jumping. A disabled button with
        // the same label holds the spot.
        <Button variant="outline" className="w-full sm:w-auto" disabled>
          <Loader2 className="size-3.5 animate-spin" />
          {t.mp.connect}
        </Button>
      )}
    </SectionFooter>
  );
}
