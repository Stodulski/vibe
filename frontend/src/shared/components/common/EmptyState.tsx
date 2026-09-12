import { useId } from 'react';
import type { LucideIcon } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';

interface EmptyStateProps {
  icon: LucideIcon;
  title: string;
  description: string;
  actionLabel?: string | undefined;
  onAction?: (() => void) | undefined;
  // Defaults to the primary (filled) style. Pass 'outline' when this empty
  // state's action duplicates a primary action already visible elsewhere on
  // the same screen (e.g. a persistent header button) — only one filled
  // primary button should exist per view.
  actionVariant?: 'default' | 'outline';
}

export function EmptyState({
  icon: Icon,
  title,
  description,
  actionLabel,
  onAction,
  actionVariant = 'default',
}: EmptyStateProps) {
  const titleId = useId();

  return (
    <section
      className="animate-fade-in flex flex-col items-center justify-center py-20 text-center"
      aria-labelledby={titleId}
    >
      <Icon className="text-text-tertiary mb-4 size-8" aria-hidden="true" />
      <h3 id={titleId} className="text-text-primary text-sm font-semibold">
        {title}
      </h3>
      {description && <p className="text-text-tertiary mt-2 max-w-xs text-sm leading-relaxed">{description}</p>}
      {actionLabel && onAction && (
        <Button onClick={onAction} variant={actionVariant} className="mt-6 min-h-12 rounded-lg" size="sm">
          {actionLabel}
        </Button>
      )}
    </section>
  );
}
