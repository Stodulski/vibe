import { Link } from 'react-router-dom';
import { ArrowRight, Shield } from 'lucide-react';
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
          <p className="text-sm text-text-secondary">{t.complex.onboardingStep3Description}</p>

          {/* The fee, in one line instead of a boxed pair. The box was a
              container around two sentences that belong together anyway, and
              the second one restated the first: "no extra platform charges" IS
              "your clients pay the service fee, you only absorb MP's". */}
          <p className="flex items-start gap-2 text-sm text-text-secondary">
            <Shield className="mt-0.5 size-4 shrink-0 text-primary-400" />
            {t.serviceFee.ownerInfo}
          </p>

          <MPConnectStatus mpConnected={mpConnected} mpAuthUrl={mpAuthUrl} onConnectClick={onConnectClick} />
        </div>
      </div>

      {/* What was decided for them, said out loud before they leave.
          The first step stopped asking for the deposit, the cancellation
          window and the services — correctly, none of them can be answered
          before there is a court. But a field removed from a form does not
          announce itself from settings: the owner would reach the dashboard
          with a 30% deposit they never chose and a public page that does not
          mention their parking. Deferring is fine; deferring in silence is
          the thing the "minimal is not simple" rule is about. */}
      <div>
        <p className="text-sm font-medium text-text-primary">{t.complex.onboardingDefaultsTitle}</p>
        <p className="mt-1 text-xs text-text-secondary">{t.complex.onboardingDefaultsBody}</p>
        <Link
          to="/settings"
          className="focus-self mt-2 inline-flex items-center gap-1 text-xs font-medium text-primary-400 underline"
        >
          {t.complex.onboardingDefaultsLink}
          <ArrowRight className="size-3" />
        </Link>
      </div>

      <MPConnectBottomNav mpConnected={mpConnected} onBack={onBack} onFinish={onFinish} />
    </div>
  );
}
