import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { setRequestLocale } from 'next-intl/server';
import {
  DailyBars,
  Delta,
  Kicker,
  SectionHeader,
  Segmented,
  StatBlock,
  Tag,
  fmtTotal,
  srButton,
} from '@/components/ui';

export const metadata: Metadata = { robots: { index: false, follow: false } };

// Illustrative series (oldest first), shaped like real repos: a launch spike
// that decays, a steady grower, and a short 3-day history.
const SPIKE = [12, 18, 25, 40, 2210, 3480, 2950, 2100, 1710, 1480, 1320, 1190, 1100, 980, 960, 900, 870, 820, 790, 760, 720, 700, 690, 650, 640, 610, 600, 590, 1240, 1610, 1429];
const STEADY = [180, 190, 175, 200, 210, 195, 205, 220, 190, 185, 230, 215, 205, 198, 240, 210, 200, 190, 225, 212, 208, 199, 230, 220, 210, 205, -30, 215, 198, 202, 195];
const SHORT = [320, 610, 880];

const SWATCHES: [string, string][] = [
  ['paper', 'bg-paper'],
  ['surface', 'bg-sr-surface'],
  ['sunk', 'bg-sunk'],
  ['rule', 'bg-rule'],
  ['bar', 'bg-bar'],
  ['ink-3', 'bg-ink-3'],
  ['ink-2', 'bg-ink-2'],
  ['ink', 'bg-ink'],
  ['signal', 'bg-signal'],
  ['signal-bar', 'bg-signal-bar'],
  ['signal-soft', 'bg-signal-soft'],
  ['signal-ink', 'bg-signal-ink'],
  ['brand', 'bg-brand'],
];

/** Dev-only gallery of design tokens v1. Not built into production routes. */
export default function DesignGalleryPage({
  params,
  searchParams,
}: {
  params: { locale: string };
  searchParams: { sort?: string };
}) {
  if (process.env.NODE_ENV === 'production') notFound();
  setRequestLocale(params.locale);
  const sort = searchParams.sort ?? 'daily';

  return (
    <main className="min-h-screen bg-paper font-text text-ink">
      <div className="mx-auto max-w-[1080px] px-4 py-10 md:px-8">
        <Kicker tone="muted">DESIGN TOKENS v1 · 数据报刊</Kicker>
        <h1 className="mt-2 font-news text-sr-h1 md:text-sr-display">StarRank 设计令牌</h1>
        <p className="mt-3 max-w-[60ch] text-sr-lead text-ink-2">
          纸与墨两种中性色，一个只给异常动量的琥珀橙。层次靠线条，不靠阴影。
        </p>

        <SectionHeader className="mt-12" title="颜色" aside="--sr-* · 深浅色自动切换" />
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 md:grid-cols-7">
          {SWATCHES.map(([name, cls]) => (
            <div key={name} className="flex flex-col gap-1.5">
              <div className={`h-14 rounded border border-rule ${cls}`} />
              <span className="font-code text-sr-caption text-ink-2">{name}</span>
            </div>
          ))}
        </div>

        <SectionHeader className="mt-12" title="字体" />
        <div className="flex flex-col gap-4">
          <p className="font-news text-sr-h2">Newsreader 标题 · 今日涨幅最大的项目</p>
          <p className="text-sr-body">Inter 正文 · 收录满 7 天的项目里，今日新增 star 最多的一个上头条。</p>
          <p className="font-code text-sr-small text-ink-2">JetBrains Mono · npx skills add owner/name</p>
          <p className="text-sr-caption text-ink-3">CAPTION · 数据取自线上接口</p>
        </div>

        <SectionHeader className="mt-12" title="增量" aside="▲▼ 表方向，不用红绿" />
        <div className="flex flex-wrap items-baseline gap-6 text-sr-lead">
          <Delta value={1429} label="今日 1,429 star" />
          <Delta value={195} label="今日 195 star" />
          <Delta value={-1306} label="今日减少 1,306 star" />
          <Delta value={null} label="暂无数据" />
        </div>

        <SectionHeader className="mt-12" title="每日增量柱" aside="31 天 · 最后一根是今天" />
        <div className="grid gap-6 md:grid-cols-3">
          {[
            ['发布冲高后回落', SPIKE],
            ['稳定增长（含一天负值）', STEADY],
            ['只收录了 3 天', SHORT],
          ].map(([title, values]) => (
            <div key={title as string} className="border-t border-rule pt-3">
              <p className="mb-2 text-sr-small text-ink-2">{title as string}</p>
              <DailyBars values={values as number[]} label={`每日新增 star，${title as string}`} height={40} />
            </div>
          ))}
        </div>

        <SectionHeader className="mt-12" title="数字块" />
        <div className="flex flex-wrap gap-10">
          <StatBlock size="lg" tone="signal" label="今日" value="▲1,749" sub="收录满 7 天里最高" />
          <StatBlock label="总 star" value={fmtTotal(471630)} />
          <StatBlock label="7 天" value="+12,804" />
          <StatBlock size="sm" label="名次" value="#3" />
        </div>

        <SectionHeader className="mt-12" title="控件" />
        <div className="flex flex-col gap-5">
          <Segmented
            label="排序"
            items={[
              { key: 'daily', label: '今日', href: '/design?sort=daily', active: sort === 'daily' },
              { key: 'weekly', label: '7 天', href: '/design?sort=weekly', active: sort === 'weekly' },
              { key: 'stars', label: '总量', href: '/design?sort=stars', active: sort === 'stars' },
            ]}
          />
          <div className="flex flex-wrap gap-3">
            <button type="button" className={srButton('solid')}>主要操作</button>
            <button type="button" className={srButton('outline')}>次要操作</button>
            <button type="button" className={srButton('quiet')}>安静按钮</button>
          </div>
          <div className="flex flex-wrap gap-2">
            <Tag>TypeScript</Tag>
            <Tag>AI 智能体</Tag>
            <Tag tone="signal">新面孔</Tag>
          </div>
          <div className="w-fit rounded-lg border border-rule bg-sr-surface p-4 shadow-sr-pop text-sr-small text-ink-2">
            浮层是唯一带阴影的东西（sr-pop）
          </div>
        </div>
      </div>
    </main>
  );
}
