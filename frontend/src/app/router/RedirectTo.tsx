import { Navigate, useLocation, useParams, type Params } from 'react-router-dom';

/** Replaces the entry with `to(params)`, keeping the query string and hash. */
export function RedirectTo({ to }: { to: (params: Params) => string }) {
  const params = useParams();
  const { search, hash } = useLocation();
  return <Navigate to={{ pathname: to(params), search, hash }} replace />;
}
