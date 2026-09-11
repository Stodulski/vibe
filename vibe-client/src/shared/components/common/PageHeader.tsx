import type { LucideIcon } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { usePageHeading } from '@/shared/components/layout/page-heading/usePageHeading';
import { cn } from '@/shared/lib/utils';

interface PageHeaderProps {
  title: string;
  description?: string;
  action?: {
    label: string;
    onClick: () => void;
    icon?: LucideIcon;
    disabled?: boolean;
  };
}

/**
 * The title/description render here only on mobile (`md:hidden`) — on desktop
 * the title is published to the shell's masthead via `usePageHeading` instead
 * (paired there with the live date/time, the same on every page), so it sits
 * in line with the logo rather than repeating inside every page.
 */
export function PageHeader({ title, description, action }: PageHeaderProps) {
  usePageHeading(title);

  return (
    <div
      className={cn(
        'mb-3 flex flex-col gap-2.5 sm:mb-5 sm:flex-row sm:items-center sm:justify-between',
        !action && 'md:hidden',
      )}
    >
      <div className="min-w-0 md:hidden">
        <h1 className="font-display truncate text-xl font-semibold leading-tight tracking-tight text-text-primary sm:text-2xl">
          {title}
        </h1>
        {description && <p className="mt-1 truncate whitespace-nowrap text-sm text-text-tertiary">{description}</p>}
      </div>
      {action && (
        <Button
          onClick={action.onClick}
          disabled={action.disabled}
          className="w-full shrink-0 gap-2 rounded-xl sm:w-auto md:ml-auto"
        >
          {action.icon && <action.icon className="size-4" aria-hidden="true" />}
          {action.label}
        </Button>
      )}
    </div>
  );
}
