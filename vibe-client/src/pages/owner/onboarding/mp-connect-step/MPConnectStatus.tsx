import { ExternalLink, CheckCircle2, Loader2 } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface MPConnectStatusProps {
  mpConnected: boolean;
  mpAuthUrl: string | null;
  onConnectClick: () => void;
}

export function MPConnectStatus({ mpConnected, mpAuthUrl, onConnectClick }: MPConnectStatusProps) {
  if (mpConnected) {
    return (
      <div className="flex items-center gap-3 rounded-xl border border-primary-500/20 bg-primary-500/5 p-4">
        <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-primary-500/20">
          <CheckCircle2 className="size-5 text-primary-400" />
        </div>
        <span className="font-medium text-text-primary">{t.mp.connected}</span>
      </div>
    );
  }

  if (mpAuthUrl) {
    return (
      <Button asChild size="lg" className="w-full">
        <a href={mpAuthUrl} onClick={onConnectClick}>
          <ExternalLink className="size-4" />
          {t.mp.connect}
        </a>
      </Button>
    );
  }

  return (
    <div className="flex items-center justify-center py-4">
      <Loader2 className="size-5 animate-spin text-text-tertiary" />
    </div>
  );
}
