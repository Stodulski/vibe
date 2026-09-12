import { Loader2 } from 'lucide-react';
import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

const sizes = {
  sm: 'size-4',
  md: 'size-6',
  lg: 'size-10',
} as const;

interface LoadingSpinnerProps {
  size?: keyof typeof sizes;
  className?: string;
  label?: string;
}

export function LoadingSpinner({ size = 'md', className, label }: LoadingSpinnerProps) {
  return (
    <div
      className={cn('flex items-center justify-center', className)}
      role="status"
      aria-label={label ?? t.common.loading}
    >
      <Loader2 className={cn('text-primary-400 animate-spin', sizes[size])} aria-hidden="true" />
    </div>
  );
}
