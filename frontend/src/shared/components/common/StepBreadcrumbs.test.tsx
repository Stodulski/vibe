import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { StepBreadcrumbs, type Crumb } from './StepBreadcrumbs';

describe('StepBreadcrumbs — one crumb', () => {
  const crumbs: Crumb[] = [{ id: 'duration', label: 'Duración', answer: '90 min' }];

  it('names the step for screen readers and shows the answer to sighted users', () => {
    render(<StepBreadcrumbs crumbs={crumbs} onEdit={vi.fn()} />);

    const button = screen.getByRole('button', { name: /Duración/ });
    expect(button).toHaveAccessibleName(/90 min/);

    const label = screen.getByText('Duración');
    expect(label).toHaveClass('sr-only');
  });

  it('calls onEdit with the crumb id when clicked', async () => {
    const user = userEvent.setup();
    const onEdit = vi.fn();
    render(<StepBreadcrumbs crumbs={crumbs} onEdit={onEdit} />);

    await user.click(screen.getByRole('button', { name: /Duración/ }));

    expect(onEdit).toHaveBeenCalledWith('duration');
  });
});

describe('StepBreadcrumbs — no crumbs', () => {
  it('renders nothing', () => {
    const { container } = render(<StepBreadcrumbs crumbs={[]} onEdit={vi.fn()} />);

    expect(container.firstChild).toBeNull();
  });
});
