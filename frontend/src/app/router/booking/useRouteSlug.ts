import { useParams } from 'react-router-dom';

/** The `:slug` of the route. The booking pages treat an empty slug as "no complex". */
export function useRouteSlug(): string {
  const { slug } = useParams<{ slug: string }>();
  return slug ?? '';
}
