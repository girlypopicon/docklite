/** @type {import('next').NextConfig} */
const agentUrl =
  process.env.AGENT_URL ||
  process.env.DOCKLITE_AGENT_URL ||
  'http://localhost:3000';
const normalizedAgentUrl = agentUrl.replace(/\/+$/, '');

const demoExport = process.env.DOCKLITE_DEMO_EXPORT === '1';

const nextConfig = {
  // The public demo can be exported as plain static files (DOCKLITE_DEMO_EXPORT=1): no Node process, nothing to attack.
  ...(demoExport ? { output: 'export', trailingSlash: true, images: { unoptimized: true } } : {}),
  // Shown in the footer and Settings; always the version in package.json (kept in step with VERSION).
  // NEXT_PUBLIC_DOCKLITE_DEMO is always defined ('0' unless building the demo) so the compiler can drop the demo code from normal builds.
  env: { NEXT_PUBLIC_DOCKLITE_VERSION: require('./package.json').version, NEXT_PUBLIC_DOCKLITE_DEMO: process.env.NEXT_PUBLIC_DOCKLITE_DEMO || '0' },
  // The demo (scripts/demo.sh) builds into its own folder so it never touches the installed build.
  distDir: process.env.NEXT_DIST_DIR || '.next',
  experimental: {
    serverComponentsExternalPackages: ['better-sqlite3', 'dockerode'],
    instrumentationHook: true,
  },
  async redirects() {
    if (process.env.NEXT_PUBLIC_DOCKLITE_DEMO !== '1' || demoExport) return [];
    // The demo has no login or debug pages.
    return ['/login', '/debug', '/debug-auth', '/emergency'].map((source) => ({ source, destination: '/', permanent: false }));
  },
  async rewrites() {
    if (process.env.NEXT_PUBLIC_DOCKLITE_DEMO === '1' || demoExport) return [];
    return [
      {
        source: '/api/:path*',
        destination: `${normalizedAgentUrl}/api/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
