// Bare shell — no site header/footer. Used by the social-card routes, which are
// fixed-size canvases meant to be screenshotted, not browsed.
export default function BareLayout({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}
