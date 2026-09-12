import { ArrowLeft, LogOut } from 'lucide-react';
import { AppHeader } from '@/shared/components/layout/AppHeader';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface OnboardingHeaderNavProps {
  step: number;
  onBack: () => void;
  onLogout: () => void;
}

export function OnboardingHeaderNav({ step, onBack, onLogout }: OnboardingHeaderNavProps) {
  return (
    <AppHeader>
      {step === 1 && (
        <Button variant="ghost" size="sm" onClick={onBack} className="text-text-tertiary">
          <ArrowLeft className="size-4" />
          <span className="hidden sm:inline">{t.common.back}</span>
          <span className="sr-only sm:hidden">{t.common.back}</span>
        </Button>
      )}
      <Button variant="ghost" size="sm" onClick={onLogout} className="text-text-tertiary">
        <LogOut className="size-4" />
        <span className="hidden sm:inline">{t.auth.logout}</span>
        <span className="sr-only sm:hidden">{t.auth.logout}</span>
      </Button>
    </AppHeader>
  );
}
