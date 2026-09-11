import { render, screen } from '@testing-library/react';
import { CheckCircle } from 'lucide-react';
import { StatusHero } from './StatusHero';

describe('StatusHero', () => {
  it('renders the title and description', () => {
    render(<StatusHero icon={CheckCircle} tone="success" title="Listo" description="Todo salió bien" />);
    expect(screen.getByRole('heading', { name: 'Listo' })).toBeInTheDocument();
    expect(screen.getByText('Todo salió bien')).toBeInTheDocument();
  });

  it('omits the description paragraph when none is passed', () => {
    render(<StatusHero icon={CheckCircle} tone="success" title="Listo" />);
    expect(screen.queryByText('Todo salió bien')).not.toBeInTheDocument();
  });

  it('accepts a node as the title, not only a string', () => {
    render(<StatusHero icon={CheckCircle} tone="success" title={<span data-testid="custom-title">Listo</span>} />);
    expect(screen.getByTestId('custom-title')).toBeInTheDocument();
  });

  it('renders children content below the description', () => {
    render(
      <StatusHero icon={CheckCircle} tone="success" title="Listo">
        <button type="button">Volver</button>
      </StatusHero>,
    );
    expect(screen.getByRole('button', { name: 'Volver' })).toBeInTheDocument();
  });
});
