import { render, screen } from '@testing-library/react';
import { VisuallyHidden } from './VisuallyHidden';

describe('VisuallyHidden', () => {
  it('renders children', () => {
    render(<VisuallyHidden>Hidden text</VisuallyHidden>);
    expect(screen.getByText('Hidden text')).toBeInTheDocument();
  });

  // The one class assertion in this codebase that earns its place: `sr-only`
  // is not styling applied to this component, it IS the component. Without it
  // there is nothing left — the whole point is text a screen reader reaches
  // and an eye does not.
  //
  // What went with the cleanup was the sibling asserting `tagName === 'SPAN'`.
  // A div here would look the same, sound the same to a screen reader, and
  // break nothing; that test could only ever fail on a refactor.
  it('applies sr-only class for screen reader visibility', () => {
    render(<VisuallyHidden>SR text</VisuallyHidden>);
    const element = screen.getByText('SR text');
    expect(element).toHaveClass('sr-only');
  });
});
