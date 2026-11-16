import type { ReactNode } from 'react';
import { ArrowLeft } from 'lucide-react';
import { StepIndicator } from './StepIndicator';
import { BOOKING_STEPS } from './checkoutSteps';

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
  children: ReactNode;
}

/**
 * The column every checkout shares, a booking's, an open play's and a
 * tournament's: the progress, a way back, then what the step holds. It takes
 * the page's full width, so the storefront column sets the gutters.
 */
export function CheckoutLayout({ currentStep, steps = BOOKING_STEPS, back, children }: CheckoutLayoutProps) {
  return (
    <div className="animate-fade-in w-full space-y-6 sm:space-y-8">
      <StepIndicator currentStep={currentStep} steps={steps} />
      <CheckoutBackLink {...back} />
      {children}
    </div>
  );
}
