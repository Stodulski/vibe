import { Link } from 'react-router-dom';
import { Menu } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Logo } from '@/shared/components/common/Logo';
import { ES_AR } from '@/shared/i18n/es_AR';

interface HeaderProps {
  onMenuClick: () => void;
  /** Logo link destination. Defaults to the owner dashboard root. */
  to?: string;
  /** Optional brand label shown next to the logo (e.g. "Admin"). */
  label?: string;
}

export function Header({ onMenuClick, to = '/', label }: HeaderProps) {
  return (
    <header className="flex h-16 shrink-0 items-center justify-between border-b border-border-subtle bg-bg-base px-4 md:hidden safe-area-inset">
      <Button
        variant="ghost"
        size="icon"
        className="size-10 shrink-0 rounded-lg"
        onClick={onMenuClick}
        aria-label={ES_AR.layout.openMenu}
      >
        <Menu className="size-5" aria-hidden="true" />
      </Button>
      <Link to={to} className="flex items-center gap-2">
        {label && <span className="text-xs font-medium text-primary-400">{label}</span>}
        <Logo />
      </Link>
    </header>
  );
}
