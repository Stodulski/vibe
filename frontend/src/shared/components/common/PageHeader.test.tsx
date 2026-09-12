import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Plus } from 'lucide-react';
import { PageHeader } from './PageHeader';

function renderWithAction(overrides: Partial<React.ComponentProps<typeof PageHeader>['action']> = {}) {
  const onClick = vi.fn();
  render(<PageHeader title="Canchas" action={{ label: 'Nueva cancha', onClick, ...overrides }} />);
  return { onClick };
}

describe('PageHeader', () => {
  it('renders the title', () => {
    render(<PageHeader title="Reservas" />);
    expect(screen.getByRole('heading', { name: 'Reservas' })).toBeInTheDocument();
  });

  it('renders the description when provided', () => {
    render(<PageHeader title="Reservas" description="Gestiona tus reservas" />);
    expect(screen.getByText('Gestiona tus reservas')).toBeInTheDocument();
  });

  // Asserted on the element the description would render into, not on how many
  // children the heading's parent happens to have — that count would also
  // change the day an unrelated sibling appears beside the title, for reasons
  // having nothing to do with the description.
  it('does not render description when not provided', () => {
    const { container } = render(<PageHeader title="Reservas" />);

    expect(screen.getByRole('heading', { name: 'Reservas' })).toBeInTheDocument();
    expect(container.querySelector('p')).not.toBeInTheDocument();
  });

  it('renders action button when action prop is provided', () => {
    renderWithAction();
    expect(screen.getByRole('button', { name: /nueva cancha/i })).toBeInTheDocument();
  });

  it('calls action onClick when button is clicked', async () => {
    const user = userEvent.setup();
    const { onClick } = renderWithAction();
    await user.click(screen.getByRole('button', { name: /nueva cancha/i }));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('renders action button with icon when provided', () => {
    renderWithAction({ icon: Plus });
    const button = screen.getByRole('button', { name: /nueva cancha/i });
    expect(button).toBeInTheDocument();
  });

  it('disables action button when disabled is true', () => {
    renderWithAction({ disabled: true });
    expect(screen.getByRole('button', { name: /nueva cancha/i })).toBeDisabled();
  });

  it('does not render action button when action prop is not provided', () => {
    render(<PageHeader title="Reservas" />);
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });
});
