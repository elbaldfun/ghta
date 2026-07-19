// Static data for the 2a ranking UI: taxonomy → GitHub search mapping,
// canonical language colors, and package-registry heuristics.
// Labels live in the i18n messages (rank.cats / rank.subs).

export interface TaxonomyNode {
  id: string;
  /** GitHub search qualifiers for this node (combined with the base query). */
  query: string;
  subs?: TaxonomyNode[];
}

/** Quality floor applied to every taxonomy listing. */
export const BASE_QUALIFIER = 'stars:>1000';

export const TAXONOMY: TaxonomyNode[] = [
  {
    id: 'frontend',
    query: 'topic:frontend',
    subs: [
      { id: 'fe-framework', query: 'topic:frontend topic:framework' },
      { id: 'fe-ui', query: 'topic:ui topic:components' },
      { id: 'fe-css', query: 'topic:css-framework' },
    ],
  },
  {
    id: 'backend',
    query: 'topic:backend',
    subs: [
      { id: 'be-web', query: 'topic:web-framework' },
      { id: 'be-db', query: 'topic:database' },
      { id: 'be-async', query: 'topic:async topic:runtime' },
    ],
  },
  {
    id: 'ai',
    query: 'topic:machine-learning',
    subs: [
      { id: 'ai-dl', query: 'topic:deep-learning' },
      { id: 'ai-nlp', query: 'topic:nlp' },
      { id: 'ai-cv', query: 'topic:computer-vision' },
    ],
  },
  {
    id: 'infra',
    query: 'topic:devops',
    subs: [
      { id: 'infra-orch', query: 'topic:kubernetes' },
      { id: 'infra-rt', query: 'topic:container topic:runtime' },
      { id: 'infra-os', query: 'topic:operating-system' },
    ],
  },
  {
    id: 'tools',
    query: 'topic:developer-tools',
    subs: [
      { id: 'tools-editor', query: 'topic:editor' },
      { id: 'tools-ssg', query: 'topic:static-site-generator' },
    ],
  },
  {
    id: 'lang',
    query: 'topic:programming-language',
    subs: [{ id: 'lang-compiler', query: 'topic:compiler' }],
  },
];

export function taxonomyQuery(cat?: string | null, sub?: string | null): string {
  const group = TAXONOMY.find((g) => g.id === cat);
  if (!group) return '';
  if (sub) {
    const node = group.subs?.find((s) => s.id === sub);
    if (node) return node.query;
  }
  return group.query;
}

/** Canonical GitHub language colors (subset + fallback). */
export const LANG_COLORS: Record<string, string> = {
  JavaScript: '#f1c40f',
  TypeScript: '#3178c6',
  Python: '#3572A5',
  Go: '#00ADD8',
  Rust: '#dea584',
  C: '#7d7d7d',
  'C++': '#f34b7d',
  'C#': '#178600',
  Java: '#b07219',
  Ruby: '#cc342d',
  PHP: '#6f7cba',
  CSS: '#663399',
  HTML: '#e34c26',
  Shell: '#89e051',
  Swift: '#F05138',
  Kotlin: '#A97BFF',
  Dart: '#00B4AB',
  Zig: '#ec915c',
  Lua: '#000080',
  Vue: '#41b883',
  Elixir: '#6e4a7e',
  Haskell: '#5e5086',
  Scala: '#c22d40',
  'Jupyter Notebook': '#DA5B0B',
  Dockerfile: '#384d54',
  'Vim Script': '#199f4b',
};

export function langColor(language: string | null | undefined): string {
  if (!language) return '#8b949e';
  return LANG_COLORS[language] ?? '#8b949e';
}

/** Languages offered in the fine-grained filter dropdown. */
export const FILTER_LANGS = [
  'JavaScript',
  'TypeScript',
  'Python',
  'Go',
  'Rust',
  'C',
  'C++',
  'Java',
  'Ruby',
  'PHP',
  'CSS',
] as const;

/** SPDX ids offered in the license filter dropdown (value = GitHub qualifier). */
export const FILTER_LICENSES = ['mit', 'apache-2.0', 'gpl-3.0', 'gpl-2.0', 'bsd-3-clause', 'mpl-2.0'] as const;

export const SORT_OPTIONS = ['stars', 'forks', 'updated'] as const;
export type SortOption = (typeof SORT_OPTIONS)[number];

// ---- Package/artifact heuristics (from the approved prototype) ----

const REGISTRY_BY_LANG: Record<string, string> = {
  JavaScript: 'npm',
  TypeScript: 'npm',
  CSS: 'npm',
  Python: 'PyPI',
  Go: 'Go Modules',
  Rust: 'crates.io',
  Ruby: 'RubyGems',
  PHP: 'Packagist',
  Java: 'Maven',
};

export function artifactOf(language: string | null | undefined): { has: boolean; registry: string | null } {
  const registry = (language && REGISTRY_BY_LANG[language]) || null;
  return { has: !!registry, registry };
}

export function installCmd(owner: string, name: string, language: string | null | undefined): string {
  switch (language) {
    case 'JavaScript':
    case 'TypeScript':
    case 'CSS':
      return `npm install ${name.toLowerCase()}`;
    case 'Python':
      return `pip install ${name.toLowerCase()}`;
    case 'Go':
      return `go get github.com/${owner}/${name}`;
    case 'Rust':
      return `cargo add ${name.toLowerCase()}`;
    case 'Ruby':
      return `gem install ${name.toLowerCase()}`;
    case 'PHP':
      return `composer require ${owner}/${name}`.toLowerCase();
    default:
      return `git clone https://github.com/${owner}/${name}.git`;
  }
}

export function formatCompact(n: number): string {
  if (Math.abs(n) >= 1_000_000) return (n / 1_000_000).toFixed(1).replace(/\.0$/, '') + 'M';
  if (Math.abs(n) >= 1_000) return (n / 1_000).toFixed(1).replace(/\.0$/, '') + 'k';
  return String(n);
}

/** Hostname for the homepage link row (design shows bare host). */
export function homepageHost(url: string | null | undefined): string | null {
  if (!url) return null;
  try {
    const u = new URL(url.startsWith('http') ? url : `https://${url}`);
    return u.hostname.replace(/^www\./, '') + (u.pathname !== '/' ? u.pathname : '');
  } catch {
    return null;
  }
}
