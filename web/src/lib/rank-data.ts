// Static data for the 2a ranking UI: taxonomy → GitHub search mapping,
// canonical language colors, and package-registry heuristics.
// Labels live in the i18n messages (rank.cats / rank.subs).

export interface TaxonomyNode {
  id: string;
  /** Topic tags a repo must carry (matched against sourceData.topicNames). */
  topics: string[];
  subs?: TaxonomyNode[];
}

export const TAXONOMY: TaxonomyNode[] = [
  {
    id: 'frontend',
    topics: ['frontend'],
    subs: [
      { id: 'fe-framework', topics: ['frontend', 'framework'] },
      { id: 'fe-ui', topics: ['ui', 'components'] },
      { id: 'fe-css', topics: ['css-framework'] },
    ],
  },
  {
    id: 'backend',
    topics: ['backend'],
    subs: [
      { id: 'be-web', topics: ['web-framework'] },
      { id: 'be-db', topics: ['database'] },
      { id: 'be-async', topics: ['async', 'runtime'] },
    ],
  },
  {
    id: 'ai',
    topics: ['machine-learning'],
    subs: [
      { id: 'ai-dl', topics: ['deep-learning'] },
      { id: 'ai-nlp', topics: ['nlp'] },
      { id: 'ai-cv', topics: ['computer-vision'] },
    ],
  },
  {
    id: 'infra',
    topics: ['devops'],
    subs: [
      { id: 'infra-orch', topics: ['kubernetes'] },
      { id: 'infra-rt', topics: ['container', 'runtime'] },
      { id: 'infra-os', topics: ['operating-system'] },
    ],
  },
  {
    id: 'tools',
    topics: ['developer-tools'],
    subs: [
      { id: 'tools-editor', topics: ['editor'] },
      { id: 'tools-ssg', topics: ['static-site-generator'] },
    ],
  },
  {
    id: 'lang',
    topics: ['programming-language'],
    subs: [{ id: 'lang-compiler', topics: ['compiler'] }],
  },
];

export function taxonomyTopics(cat?: string | null, sub?: string | null): string[] {
  const group = TAXONOMY.find((g) => g.id === cat);
  if (!group) return [];
  if (sub) {
    const node = group.subs?.find((s) => s.id === sub);
    if (node) return node.topics;
  }
  return group.topics;
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

/** License filter options: SPDX-ish id -> the exact name stored in the database. */
export const LICENSE_NAMES: Record<string, string> = {
  mit: 'MIT License',
  'apache-2.0': 'Apache License 2.0',
  'gpl-3.0': 'GNU General Public License v3.0',
  'gpl-2.0': 'GNU General Public License v2.0',
  'bsd-3-clause': 'BSD 3-Clause "New" or "Revised" License',
  'mpl-2.0': 'Mozilla Public License 2.0',
};
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
