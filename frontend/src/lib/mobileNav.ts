export const MOBILE_NAV_OPEN_EVENT = "agenthub:open-mobile-nav";

export function openMobileNav() {
  window.dispatchEvent(new Event(MOBILE_NAV_OPEN_EVENT));
}
