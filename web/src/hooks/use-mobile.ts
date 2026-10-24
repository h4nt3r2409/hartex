import * as React from "react";

const MOBILE_BREAKPOINT = 768;
// When the sidebar stays expanded and the content area narrows, tables and cards overlap (768-1279px range).
// Below this width, the sidebar automatically collapses to icon mode to reclaim content width.
export const TABLET_BREAKPOINT = 1280;

function useBreakpoint(breakpoint: number) {
  const [below, setBelow] = React.useState<boolean | undefined>(undefined);

  React.useEffect(() => {
    const mql = window.matchMedia(`(max-width: ${breakpoint - 1}px)`);
    const onChange = () => setBelow(window.innerWidth < breakpoint);
    mql.addEventListener("change", onChange);
    setBelow(window.innerWidth < breakpoint);
    return () => mql.removeEventListener("change", onChange);
  }, [breakpoint]);

  return below;
}

export function useIsMobile() {
  return !!useBreakpoint(MOBILE_BREAKPOINT);
}

// Whether the viewport is tablet width (768-1279px). On the initial render (before measurement),
// undefined is returned as-is so callers can distinguish "not known yet" from "false".
export function useIsTablet() {
  return useBreakpoint(TABLET_BREAKPOINT);
}
