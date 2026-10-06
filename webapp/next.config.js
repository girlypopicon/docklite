/** @type {import('next').NextConfig} */
const agentUrl =
  process.env.AGENT_URL ||
  process.env.DOCKLITE_AGENT_URL ||
  'http://localhost:3000';
const normalizedAgentUrl = agentUrl.replace(/\/+$/, '');

const nextConfig = {
  // Shown in the footer and Settings; always the version in package.json (kept in step with VERSION).
  env: { NEXT_PUBLIC_DOCKLITE_VERSION: require('./package.json').version },
  // The demo (scripts/demo.sh) builds into its own folder so it never touches the installed build.
  distDir: process.env.NEXT_DIST_DIR || '.next',
  experimental: {
    serverComponentsExternalPackages: ['better-sqlite3', 'dockerode'],
    instrumentationHook: true,
  },
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: `${normalizedAgentUrl}/api/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
