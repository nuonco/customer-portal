import { useEffect } from "react";
import { useLocation } from "react-router";

/**
 * BackendPage navigates the browser to the Go-rendered version of the current
 * URL by prepending the /bff proxy prefix. Used for routes that are still
 * server-rendered (Templ/HTMX) so the React router doesn't intercept them.
 */
export const BackendPage = () => {
  const { pathname, search, hash } = useLocation();

  useEffect(() => {
    window.location.replace(`/bff${pathname}${search}${hash}`);
  }, [pathname, search, hash]);

  return null;
};
