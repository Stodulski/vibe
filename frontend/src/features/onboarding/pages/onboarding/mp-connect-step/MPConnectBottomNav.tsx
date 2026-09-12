import { ArrowRight, ArrowLeft } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface MPConnectBottomNavProps {
  mpConnected: boolean;
  onBack: () => void;
  onFinish: () => void;
}

export function MPConnectBottomNav({ mpConnected, onBack, onFinish }: MPConnectBottomNavProps) {
  return (
    // No surface of its own, at any width — the steps above it have none
    // either, and a bordered bar under borderless content reads as a footer
    // for something that never opened. The rule above it does the separating.
    <div className="border-border-subtle flex items-center justify-between gap-3 border-t pt-5">
      <Button variant="ghost" size="sm" onClick={onBack} className="text-text-tertiary">
        <ArrowLeft className="size-4" />
        {t.common.back}
      </Button>
      {/* Always here. It used to render only when MercadoPago was connected —
          not disabled, absent — and the header's escape to /complexes only
          shows on step 1. Between the two, an owner who takes cash reached the
          end of onboarding and had exactly two ways out: back, or log out.
          A club can be bookable without online payment; the connection is the
          one thing on this screen that depends on somebody else's website.

          Connected it is the primary and the only action. Not connected it is
          secondary: "Conectar" leads, this one leaves without it. */}
      <Button
        onClick={onFinish}
        size="lg"
        variant={mpConnected ? 'default' : 'ghost'}
        className={mpConnected ? undefined : 'text-text-secondary'}
      >
        {mpConnected ? t.complex.onboardingFinish : t.complex.onboardingSkipPayments}
        <ArrowRight className="size-4" />
      </Button>
    </div>
  );
}
