import createNextIntlPlugin from 'next-intl/plugin';

const withNextIntl = createNextIntlPlugin('./src/i18n/request.ts');

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Self-hosted Docker deploy (deploy/docker-compose.yml): emit a minimal
  // server.js + traced node_modules instead of relying on `next start`.
  output: 'standalone',
};

export default withNextIntl(nextConfig);
