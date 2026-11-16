/**
 * The brand-green band of the storefront: the complex's header on its page and
 * the checkout's column on the booking flow.
 *
 * Below `lg` the green bleeds to the screen edges and up to the navbar: the
 * negative margins cancel the page gutter (`px-4 py-5`, `sm:p-8` on <main>), and
 * the same padding puts the content back where it was. At `lg` it is a rounded,
 * bordered card inside the page.
 *
 * The green is applied by re-pointing the theme's text, accent and rule colours
 * inside the block, so every child takes the near-black ink without a variant of
 * its own. Anything that must keep its own colour, such as a cover or a logo
 * tile, sets it on itself.
 */
export const BRAND_BAND_CLASS = [
  'bg-primary-500 relative font-medium',
  '-mx-4 -mt-5 rounded-b-3xl px-4 pt-5 pb-6 sm:-mx-8 sm:-mt-8 sm:px-8 sm:pt-8 sm:pb-10',
  'lg:m-0 lg:rounded-2xl lg:border lg:p-7 lg:border-primary-500',
  '[--color-text-primary:var(--color-text-inverse)]',
  '[--color-text-secondary:var(--color-text-inverse)]',
  '[--color-text-tertiary:var(--color-text-inverse)]',
  '[--color-primary-400:var(--color-text-inverse)]',
  '[--color-border-subtle:rgb(11_11_11/0.2)]',
].join(' ');
