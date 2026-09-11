import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Inbox } from 'lucide-react';
import { EmptyState } from './EmptyState';

function renderEmptyState(props: Partial<React.ComponentProps<typeof EmptyState>> = {}) {
  return render(<EmptyState icon={Inbox} title="Sin datos" description="Nada que ver" {...props} />);
}

describe('EmptyState', () => {
  it('renders title and description', () => {
    renderEmptyState({ title: 'No hay resultados', description: 'Intenta con otros filtros' });
    expect(screen.getByText('No hay resultados')).toBeInTheDocument();
    expect(screen.getByText('Intenta con otros filtros')).toBeInTheDocument();
  });

  it('renders with accessible label from title', () => {
    renderEmptyState({ title: 'Vacio', description: 'Sin datos' });
    expect(screen.getByRole('region', { name: 'Vacio' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Vacio' })).toBeInTheDocument();
  });

  it('renders action button when actionLabel and onAction are provided', () => {
    const onAction = vi.fn();
    renderEmptyState({
      title: 'Sin canchas',
      description: 'Crea tu primera cancha',
      actionLabel: 'Crear cancha',
      onAction,
    });
    expect(screen.getByRole('button', { name: 'Crear cancha' })).toBeInTheDocument();
  });

  it('calls onAction when action button is clicked', async () => {
    const user = userEvent.setup();
    const onAction = vi.fn();
    renderEmptyState({
      title: 'Sin canchas',
      description: 'Crea tu primera cancha',
      actionLabel: 'Crear cancha',
      onAction,
    });

    await user.click(screen.getByRole('button', { name: 'Crear cancha' }));
    expect(onAction).toHaveBeenCalledTimes(1);
  });

  it('does not render action button when only actionLabel is provided without onAction', () => {
    renderEmptyState({ actionLabel: 'Accion' });
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('does not render action button when neither actionLabel nor onAction are provided', () => {
    renderEmptyState();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('defaults the action button to the primary variant', () => {
    renderEmptyState({ actionLabel: 'Crear cancha', onAction: vi.fn() });
    expect(screen.getByRole('button', { name: 'Crear cancha' })).toHaveAttribute('data-variant', 'default');
  });

  it('renders the action button as outline when actionVariant is set, for a screen that already has a primary action elsewhere', () => {
    renderEmptyState({ actionLabel: 'Crear cancha', onAction: vi.fn(), actionVariant: 'outline' });
    expect(screen.getByRole('button', { name: 'Crear cancha' })).toHaveAttribute('data-variant', 'outline');
  });
});
