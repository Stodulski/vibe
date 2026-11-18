import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { useSelectedComplex } from '@/features/complex';
import { makeComplex } from '@/test/factories';

vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({
  useSelectedComplex: vi.fn(),
}));

import SocialKitPage from './SocialKitPage';

const KIT_URL = 'https://vibe.com.ar/kit/c/los-alamos';

function withComplex(complex: ReturnType<typeof makeComplex> | null) {
  vi.mocked(useSelectedComplex).mockReturnValue({ complex } as unknown as ReturnType<typeof useSelectedComplex>);
}

describe('SocialKitPage', () => {
  it('shows the title and the intro', () => {
    withComplex(makeComplex({ slug: 'los-alamos' }));
    render(<SocialKitPage />);
    expect(screen.getByRole('heading', { level: 1, name: 'Kit para redes' })).toBeInTheDocument();
    expect(
      screen.getByText(
        'Descargá estas imágenes y subilas a tus historias y posts. Llevan el link y el QR de tu complejo.',
      ),
    ).toBeInTheDocument();
  });

  it('titles each piece and labels each card with its format', () => {
    withComplex(makeComplex({ slug: 'los-alamos' }));
    render(<SocialKitPage />);
    expect(screen.getByRole('heading', { level: 2, name: 'Link de reservas' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 2, name: 'Cómo reservar' })).toBeInTheDocument();
    expect(screen.getAllByRole('heading', { level: 3, name: 'Historia' })).toHaveLength(2);
    expect(screen.getAllByRole('heading', { level: 3, name: 'Post' })).toHaveLength(2);
  });

  it('previews each piece in both formats with the landing image as source', () => {
    withComplex(makeComplex({ slug: 'los-alamos' }));
    render(<SocialKitPage />);
    expect(screen.getByRole('img', { name: 'Historia: Link de reservas' })).toHaveAttribute(
      'src',
      `${KIT_URL}/reservar-historia.png`,
    );
    expect(screen.getByRole('img', { name: 'Post: Link de reservas' })).toHaveAttribute(
      'src',
      `${KIT_URL}/reservar-post.png`,
    );
    expect(screen.getByRole('img', { name: 'Historia: Cómo reservar' })).toHaveAttribute(
      'src',
      `${KIT_URL}/como-reservar-historia.png`,
    );
    expect(screen.getByRole('img', { name: 'Post: Cómo reservar' })).toHaveAttribute(
      'src',
      `${KIT_URL}/como-reservar-post.png`,
    );
  });
});

describe('SocialKitPage loading and downloads', () => {
  it('lazy-loads the previews', () => {
    withComplex(makeComplex({ slug: 'los-alamos' }));
    render(<SocialKitPage />);
    for (const img of screen.getAllByRole('img')) {
      expect(img).toHaveAttribute('loading', 'lazy');
    }
  });

  it('gives each image a Descargar link to the attachment variant', () => {
    withComplex(makeComplex({ slug: 'los-alamos' }));
    render(<SocialKitPage />);
    const links = screen.getAllByRole('link', { name: /Descargar/ });
    expect(links.map((link) => link.getAttribute('href'))).toEqual([
      `${KIT_URL}/reservar-historia.png?descargar=1`,
      `${KIT_URL}/reservar-post.png?descargar=1`,
      `${KIT_URL}/como-reservar-historia.png?descargar=1`,
      `${KIT_URL}/como-reservar-post.png?descargar=1`,
    ]);
  });

  it('renders nothing while there is no selected complex', () => {
    withComplex(null);
    const { container } = render(<SocialKitPage />);
    expect(container).toBeEmptyDOMElement();
  });
});
