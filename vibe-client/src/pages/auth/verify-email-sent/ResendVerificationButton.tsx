import { Loader2 } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

function formatTime(s: number) {
  return `${String(Math.floor(s / 60))}:${String(s % 60).padStart(2, '0')}`;
}

interface ResendVerificationButtonProps {
  resending: boolean;
  cooldown: number;
  onResend: () => void;
}

export function ResendVerificationButton({ resending, cooldown, onResend }: ResendVerificationButtonProps) {
  return (
    <Button
      onClick={onResend}
      disabled={resending || cooldown > 0}
      variant="outline"
      className="mb-3 w-full rounded-full"
    >
      {resending ? (
        <Loader2 className="size-4 animate-spin" />
      ) : cooldown > 0 ? (
        `${t.auth.resendVerification} (${formatTime(cooldown)})`
      ) : (
        t.auth.resendVerification
      )}
    </Button>
  );
}
