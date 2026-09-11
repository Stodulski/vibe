import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Header } from './Header';
import { ES_AR } from '@/shared/i18n/es_AR';

vi.mock('@/shared/components/ui/button', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return { Button: passthrough('button') };
});

describe('Header', () => {
  const mockOnMenuClick = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the Vibe logo linking home', () => {
    render(
      <MemoryRouter>
        <Header onMenuClick={mockOnMenuClick} />
      </MemoryRouter>,
    );
    expect(screen.getByAltText('Vibe')).toBeInTheDocument();
  });

  it('renders menu button with correct aria-label', () => {
    render(
      <MemoryRouter>
        <Header onMenuClick={mockOnMenuClick} />
      </MemoryRouter>,
    );
    const menuBtn = screen.getByLabelText(ES_AR.layout.openMenu);
    expect(menuBtn).toBeInTheDocument();
  });

  it('calls onMenuClick when menu button is clicked', () => {
    render(
      <MemoryRouter>
        <Header onMenuClick={mockOnMenuClick} />
      </MemoryRouter>,
    );
    fireEvent.click(screen.getByLabelText(ES_AR.layout.openMenu));
    expect(mockOnMenuClick).toHaveBeenCalledTimes(1);
  });

  it('links to / by default', () => {
    render(
      <MemoryRouter>
        <Header onMenuClick={mockOnMenuClick} />
      </MemoryRouter>,
    );
    expect(screen.getByAltText('Vibe').closest('a')).toHaveAttribute('href', '/');
  });

  it('links to a custom destination and renders the label, when given (admin usage)', () => {
    render(
      <MemoryRouter>
        <Header onMenuClick={mockOnMenuClick} to="/admin" label="Admin" />
      </MemoryRouter>,
    );
    expect(screen.getByText('Admin').closest('a')).toHaveAttribute('href', '/admin');
  });
});
