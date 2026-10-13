/**
 * Developer-app leaves: domain leaves outside the apps/ branch that the store
 * also files GUI apps under (service.DevShelves on the backend). They are a
 * view (/apps?shelf=dev), not consumer shelves, so their i18n keys carry a
 * `dev` segment to stay clear of same-named consumer shelves.
 */
export const DEV_SHELVES = [
  'devtools/terminal', 'devtools/editors', 'devtools/api-tools', 'devtools/vcs', 'devtools/package-manager', 'devtools/code-quality',
  'data/database', 'infra/containers', 'infra/automation', 'infra/monitoring', 'gamedev/engines', 'security/offensive', 'agents/coding',
];

/** i18n key (namespace `rank`) for a shelf value as the API returns it. */
export function shelfLabelKey(shelf: string): string {
  const slug = DEV_SHELVES.includes(shelf) ? `dev/${shelf}` : shelf;
  return `shelf_${slug.replace(/[/-]/g, '_')}`;
}
