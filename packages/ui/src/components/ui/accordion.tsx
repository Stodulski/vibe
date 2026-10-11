'use client';

import * as React from 'react';
import { ChevronDownIcon } from 'lucide-react';
import { Accordion as AccordionPrimitive } from 'radix-ui';

import { cn } from '../../lib/cn';

function Accordion({ ...props }: React.ComponentProps<typeof AccordionPrimitive.Root>) {
  return <AccordionPrimitive.Root data-slot="accordion" {...props} />;
}

function AccordionItem({ className, ...props }: React.ComponentProps<typeof AccordionPrimitive.Item>) {
  return (
    <AccordionPrimitive.Item
      data-slot="accordion-item"
      className={cn('border-border-subtle border-b last:border-b-0', className)}
      {...props}
    />
  );
}

/**
 * Radix wraps the trigger in a header that renders as h3. A page whose outline
 * needs h2 there passes `headingLevel={2}`; the default stays h3 for every
 * existing caller.
 */
function AccordionTrigger({
  className,
  children,
  headingLevel = 3,
  ...props
}: React.ComponentProps<typeof AccordionPrimitive.Trigger> & { headingLevel?: 2 | 3 | undefined }) {
  const Heading = headingLevel === 2 ? 'h2' : 'h3';
  return (
    <AccordionPrimitive.Header asChild>
      <Heading className="flex">
        <AccordionPrimitive.Trigger
          data-slot="accordion-trigger"
          className={cn(
            // 48px tall: this is a touch target, and the minimum is 48x48.
            'text-text-tertiary hover:text-text-secondary [&[data-state=open]]:text-text-secondary flex flex-1 cursor-pointer items-center justify-between gap-4 py-3 text-left text-xs font-semibold tracking-wide uppercase transition-colors [&[data-state=open]>svg]:rotate-180',
            className,
          )}
          {...props}
        >
          {children}
          <ChevronDownIcon className="size-4 shrink-0 transition-transform duration-200" aria-hidden="true" />
        </AccordionPrimitive.Trigger>
      </Heading>
    </AccordionPrimitive.Header>
  );
}

function AccordionContent({ className, children, ...props }: React.ComponentProps<typeof AccordionPrimitive.Content>) {
  return (
    <AccordionPrimitive.Content
      data-slot="accordion-content"
      // Radix measures the panel and publishes its height as a CSS variable,
      // which is what makes a height animation possible at all — `height:auto`
      // is not animatable. The keyframes are defined in globals.css.
      className="data-[state=closed]:animate-accordion-up data-[state=open]:animate-accordion-down overflow-hidden"
      {...props}
    >
      <div className={cn('pb-4', className)}>{children}</div>
    </AccordionPrimitive.Content>
  );
}

export { Accordion, AccordionItem, AccordionTrigger, AccordionContent };
