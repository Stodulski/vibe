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
      className="flex flex-col items-center justify-center py-20 text-center animate-fade-in"
      aria-labelledby={titleId}
    >
      <Icon className="mb-4 size-8 text-text-tertiary" aria-hidden="true" />
      <h3 id={titleId} className="text-sm font-semibold text-text-primary">
        {title}
      </h3>
      {description && <p className="mt-2 max-w-xs text-sm leading-relaxed text-text-tertiary">{description}</p>}
      {actionLabel && onAction && (
        <Button onClick={onAction} variant={actionVariant} className="mt-6 min-h-12 rounded-lg" size="sm">
          {actionLabel}
        </Button>
      )}
    </section>
  );
}
