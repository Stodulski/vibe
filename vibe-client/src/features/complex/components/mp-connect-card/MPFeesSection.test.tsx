import { renderWithProviders, screen, waitFor } from '@/test/test-utils';
import { MPFeesSection } from './MPFeesSection';

const validResponse = {
  vigente_desde: '2026-03-06',
  fuente: 'https://www.mercadopago.com.ar/ayuda/33399',
  iva_incluido: false,
  plazos: ['Al instante', '10 días', '18 días', '35 días'],
  grupos: [{ provincias: ['Buenos Aires', 'Chubut'], tasas: [6.6, 4.61, 3.56, 1.56] }],
  generado: '2026-09-08T20:00:00.000Z',
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('MPFeesSection', () => {
  it('renders the four plazo/rate rows for a matching province', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(validResponse) }));

    renderWithProviders(<MPFeesSection province="Buenos Aires" />);

    await waitFor(() => {
      expect(screen.getByText('Al instante')).toBeInTheDocument();
    });
    expect(screen.getByText('6,60%')).toBeInTheDocument();
    expect(screen.getByText('10 días')).toBeInTheDocument();
    expect(screen.getByText('4,61%')).toBeInTheDocument();
    expect(screen.getByText('18 días')).toBeInTheDocument();
    expect(screen.getByText('3,56%')).toBeInTheDocument();
    expect(screen.getByText('35 días')).toBeInTheDocument();
    expect(screen.getByText('1,56%')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /ver la fuente/i })).toHaveAttribute(
      'href',
      'https://www.mercadopago.com.ar/ayuda/33399',
    );
  });

  it('renders a not-found line for a province absent from every group', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(validResponse) }));

    renderWithProviders(<MPFeesSection province="Marte" />);

    await waitFor(() => {
      expect(screen.getByText(/no encontramos esta provincia/i)).toBeInTheDocument();
    });
    expect(screen.getByText(/Marte/)).toBeInTheDocument();
  });

  it('renders a calm error line when the fetch rejects', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('network down')));

    renderWithProviders(<MPFeesSection province="Buenos Aires" />);

    await waitFor(() => {
      expect(screen.getByText(/no pudimos cargar los costos de mercadopago/i)).toBeInTheDocument();
    });
  });
});
