import { Shield } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { MPConnectStatus } from './mp-connect-step/MPConnectStatus';
import { MPConnectBottomNav } from './mp-connect-step/MPConnectBottomNav';

const t = ES_AR;

interface MPConnectStepProps {
  complexId: string;
  mpConnected: boolean;
  mpAuthUrl: string | null;
  onConnectClick: () => void;
  onBack: () => void;
  onFinish: () => void;
}

export function MPConnectStep({ mpConnected, mpAuthUrl, onConnectClick, onBack, onFinish }: MPConnectStepProps) {
  return (
    <div className="space-y-4">
      <div>
        <div className="space-y-6">
          {/* No heading — the step indicator above names this step. */}
          <p className="text-text-secondary text-sm">{t.complex.onboardingStep3Description}</p>

          {/* The fee, in one line instead of a boxed pair. The box was a
              container around two sentences that belong together anyway, and
              the second one restated the first: "no extra platform charges" IS
              "your clients pay the service fee, you only absorb MP's". */}
          <p className="text-text-secondary flex items-start gap-2 text-sm">
            <Shield className="text-primary-400 mt-0.5 size-4 shrink-0" />
            {/* One line per fact. Run together, the two sentences read as a
                single paragraph about charges and the owner has to find their
                own half in it; apart, the answer to "what does this cost me" is
                on its own line. */}
            <span>
              <span className="block">{t.serviceFee.ownerInfoClients}</span>
              <span className="block">{t.serviceFee.ownerInfoYou}</span>
            </span>
          </p>

          <MPConnectStatus mpConnected={mpConnected} mpAuthUrl={mpAuthUrl} onConnectClick={onConnectClick} />

          {/* Whether this step is required, answered where the choice is made.
              A "you can already take bookings" block used to sit here, which
              was not true until payments were connected: without them the
              public page only offers the club's WhatsApp. The defaults it
              listed (30% deposit, 24 h cancellation) only apply to online
              payments, so they went with it. */}
          {!mpConnected && <p className="text-text-tertiary text-xs">{t.complex.onboardingPaymentsLater}</p>}
        </div>
      </div>

      <MPConnectBottomNav mpConnected={mpConnected} onBack={onBack} onFinish={onFinish} />
    </div>
  );
}
