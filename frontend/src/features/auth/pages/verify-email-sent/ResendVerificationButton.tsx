import { LoadingButton } from '@/shared/components/common/LoadingButton';
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
    <LoadingButton
      onClick={onResend}
      disabled={cooldown > 0}
      loading={resending}
      variant="outline"
      className="mb-3 w-full rounded-full"
    >
      {cooldown > 0 ? `${t.auth.resendVerification} (${formatTime(cooldown)})` : t.auth.resendVerification}
    </LoadingButton>
  );
}
