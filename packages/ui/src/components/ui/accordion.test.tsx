import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from './accordion';

function renderAccordion(headingLevel?: 2 | 3) {
  return render(
    <Accordion type="single" collapsible defaultValue="horarios">
      <AccordionItem value="horarios">
        <AccordionTrigger headingLevel={headingLevel}>Horarios</AccordionTrigger>
        <AccordionContent>Lunes a viernes, de 8 a 23</AccordionContent>
      </AccordionItem>
    </Accordion>,
  );
}

describe('AccordionTrigger heading level', () => {
  it('renders the header as h3 when no level is given, as existing callers expect', () => {
    renderAccordion();

    expect(screen.getByRole('heading', { level: 3, name: 'Horarios' })).toBeInTheDocument();
  });

  it('renders the header as h2 when asked, so a page outline does not skip from h1 to h3', () => {
    renderAccordion(2);

    expect(screen.getByRole('heading', { level: 2, name: 'Horarios' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { level: 3 })).not.toBeInTheDocument();
  });

  it('keeps the trigger button inside the heading and its open state intact at h2', () => {
    renderAccordion(2);

    const trigger = screen.getByRole('button', { name: 'Horarios' });
    expect(trigger.closest('h2')).not.toBeNull();
    expect(trigger).toHaveAttribute('aria-expanded', 'true');
  });
});
