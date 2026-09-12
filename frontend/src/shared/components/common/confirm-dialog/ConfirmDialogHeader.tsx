import { AlertTriangle, Info } from 'lucide-react';
import { AlertDialogDescription, AlertDialogHeader, AlertDialogTitle } from '@/shared/components/ui/alert-dialog';
import { cn } from '@/shared/lib/utils';

interface ConfirmDialogHeaderProps {
  isDestructive: boolean;
  title: string;
  description: string;
}

export function ConfirmDialogHeader({ isDestructive, title, description }: ConfirmDialogHeaderProps) {
  return (
    <AlertDialogHeader className="gap-3 sm:text-left">
      {/* Icon */}
      <div
        className={cn(
          'flex size-10 shrink-0 items-center justify-center rounded-xl',
          isDestructive ? 'bg-error-bg text-error-text' : 'bg-primary-500/10 text-primary-400',
        )}
      >
        {isDestructive ? <AlertTriangle className="size-5" /> : <Info className="size-5" />}
      </div>

      <AlertDialogTitle className="font-display text-text-primary text-base font-bold tracking-tight sm:text-lg">
        {title}
      </AlertDialogTitle>
      <AlertDialogDescription className="text-nav text-text-secondary leading-relaxed">
        {description}
      </AlertDialogDescription>
    </AlertDialogHeader>
  );
}
