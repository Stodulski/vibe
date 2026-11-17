import type { ReactNode } from 'react';
import { ArrowLeft } from 'lucide-react';
import { cn } from '@vibe/ui';
import { StepIndicator } from './StepIndicator';
import { BOOKING_STEPS } from './checkoutSteps';
import { BRAND_BAND_CLASS } from './brandBand';

const BACK_CLASS =
  'text-text-tertiary hover:text-text-secondary flex min-h-12 items-center gap-1.5 text-sm transition-colors sm:min-h-0';

export interface CheckoutBack {
  label: string;
  /** The page the step goes back to. */
  href?: string;
  /** Or the action that undoes the step, such as giving a held slot back. */
  onClick?: () => void;
}

/** The way back one step: a link to the page before, or a button that undoes the step. */
export function CheckoutBackLink({ label, href, onClick }: CheckoutBack) {
  const content = (
    <>
      <ArrowLeft className="size-3.5" />
      {label}
    </>
  );
  if (href !== undefined) {
    return (
      <a href={href} className={BACK_CLASS}>
        {content}
      </a>
    );
  }
  return (
    <button type="button" onClick={onClick} className={BACK_CLASS}>
      {content}
    </button>
  );
}

interface CheckoutLayoutProps {
  currentStep: number;
  /** The flow's labels. A booking's by default. */
  steps?: readonly string[];
  back: CheckoutBack;
  /** What the person is paying for: the ticket, set in the green band above the form. */
  summary: ReactNode;
  /** Set in the band between the way back and the summary, such as a hold's countdown. */
  aside?: ReactNode;
  /** The fields and the pay button, below the band. */
  children: ReactNode;
}

/**
 * The column every checkout shares, a booking's, an open play's and a
 * tournament's. In one brand-green band, the same band as the complex page (see
 * BRAND_BAND_CLASS), in this order: the way back at the top, then the progress,
 * what is aside (the countdown) and the ticket. The form follows below the band. It takes the page's full width, so the storefront
 * column sets the gutters.
 */
export function CheckoutLayout({
  currentStep,
  steps = BOOKING_STEPS,
  back,
  summary,
  aside,
  children,
}: CheckoutLayoutProps) {
  // gap, not space-y: the band resets its own margins at lg, which would swallow a space-y margin.
  return (
    <div className="animate-fade-in flex w-full flex-col gap-6 sm:gap-8">
      <div className={cn(BRAND_BAND_CLASS, 'space-y-6 sm:space-y-8')}>
        <CheckoutBackLink {...back} />
        <StepIndicator currentStep={currentStep} steps={steps} tone="brand" />
        {aside}
        {summary}
      </div>
      {children}
    </div>
  );
}
