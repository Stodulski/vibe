import { cn } from '@/shared/lib/utils';

type LogoSize = 'md' | 'lg';

const LOGO_SIZES: Record<LogoSize, { className: string; width: number; height: number }> = {
  md: { className: 'h-8', width: 46, height: 32 },
  lg: { className: 'h-16', width: 92, height: 64 },
};

interface LogoProps {
  size?: LogoSize;
  alt?: string;
  className?: string;
}

/**
 * The Vibe brand mark — single source of truth for the logo asset and its
 * sizes. Before this existed, every header/sidebar hand-rolled its own
 * `<img src="/logo.svg">` with its own size, and they drifted out of sync
 * (20px/24px/32px) without anyone noticing.
 */
export function Logo({ size = 'md', alt = 'Vibe', className }: LogoProps) {
  const { className: sizeClassName, width, height } = LOGO_SIZES[size];
  return (
    <img
      src="/logo.svg"
      alt={alt}
      className={cn(sizeClassName, 'w-auto shrink-0', className)}
      width={width}
      height={height}
    />
  );
}
