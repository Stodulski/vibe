import { LogOut } from 'lucide-react';
import { AppHeader } from '@/shared/components/layout/AppHeader';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface OnboardingHeaderNavProps {
  onLogout: () => void;
}

// The account owns at most one complex, so step 1 no longer has another
// complex to go "back" to — this header carries only the logout action now.
export function OnboardingHeaderNav({ onLogout }: OnboardingHeaderNavProps) {
  return (
    <AppHeader>
      <Button variant="ghost" size="sm" onClick={onLogout} className="text-text-tertiary">
        <LogOut className="size-4" />
        <span className="hidden sm:inline">{t.auth.logout}</span>
        <span className="sr-only sm:hidden">{t.auth.logout}</span>
      </Button>
    </AppHeader>
  );
}
