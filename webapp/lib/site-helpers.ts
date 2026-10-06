import { getUserById } from './db';
import fs from 'fs/promises';

export const SITES_BASE_DIR = process.env.DOCKLITE_SITES_DIR || '/var/www/sites';

/**
 * Get the site directory path for a user and domain
 * Format: /var/www/sites/{username}/{domain}/
 */
export function getSitePath(username: string, domain: string): string {
  return `${SITES_BASE_DIR}/${username}/${domain}`;
}

/**
 * Get the site path by user ID and domain
 */
export function getSitePathByUserId(userId: number, domain: string): string {
  const user = getUserById(userId);
  if (!user) {
    throw new Error('User not found');
  }
  return getSitePath(user.username, domain);
}

/**
 * Create the directory structure for a site
 * Creates: /var/www/sites/{username}/{domain}/
 * Sets appropriate permissions
 */
export async function createSiteDirectory(username: string, domain: string): Promise<string> {
  const sitePath = getSitePath(username, domain);
  console.log(`Attempting to create site directory at: ${sitePath}`);

  try {
    await fs.mkdir(sitePath, { recursive: true, mode: 0o755 });
    console.log(`✓ Successfully created directory.`);

    try {
      await fs.chmod(sitePath, 0o755);
      console.log(`✓ Successfully set directory permissions.`);
    } catch (error) {
      console.warn(`Failed to chmod ${sitePath}, continuing:`, error);
    }

    if (typeof process.getuid === 'function' && typeof process.getgid === 'function') {
      try {
        await fs.chown(sitePath, process.getuid(), process.getgid());
        console.log(`✓ Successfully changed directory ownership.`);
      } catch (error) {
        console.warn(`Failed to chown ${sitePath}, continuing:`, error);
      }
    }

    console.log(`✓ Site directory setup complete: ${sitePath}`);
    return sitePath;
  } catch (error) {
    console.error(`Error creating site directory ${sitePath}:`, error);
    throw new Error(`Failed to create site directory: ${error}`);
  }
}

/**
 * Create a default index.html for a new site
 */
export async function createDefaultIndexFile(sitePath: string, domain: string, type: 'static' | 'php' | 'node'): Promise<void> {
  let content = '';

  switch (type) {
    case 'static':
      content = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${domain}</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        h1 { color: #333; }
        .info { background: #f0f0f0; padding: 15px; border-radius: 5px; }
    </style>
</head>
<body>
    <h1>Welcome to ${domain}</h1>
    <div class="info">
        <p>This is a static site managed by DockLite.</p>
        <p><strong>Site Path:</strong> <code>${sitePath}</code></p>
        <p>You can edit this file to customize your site.</p>
    </div>
</body>
</html>`;
      await fs.writeFile(`${sitePath}/index.html`, content);
      break;

    case 'php':
      content = `<?php
/**
 * ${domain}
 * PHP site managed by DockLite
 * Site path: ${sitePath}
 */
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title><?php echo '${domain}'; ?></title>
</head>
<body>
    <h1>Welcome to <?php echo '${domain}'; ?></h1>
    <p>This is a PHP site managed by DockLite.</p>
</body>
</html>`;
      await fs.writeFile(`${sitePath}/index.php`, content);
      break;

    case 'node':
      // Create package.json
      const packageJson = {
        name: domain.replace(/\./g, '-'),
        version: '1.0.0',
        description: `Node.js site for ${domain}`,
        main: 'index.js',
        scripts: {
          start: 'node index.js'
        }
      };
      await fs.writeFile(`${sitePath}/package.json`, JSON.stringify(packageJson, null, 2));

      // Create index.js
      content = `const http = require('http');

const hostname = '0.0.0.0';
const port = process.env.PORT || 3000;

const server = http.createServer((req, res) => {
  res.statusCode = 200;
  res.setHeader('Content-Type', 'text/html');
  res.end(\`
    <!DOCTYPE html>
    <html>
      <head><title>${domain}</title></head>
      <body>
        <h1>Welcome to ${domain}</h1>
        <p>Node.js site managed by DockLite</p>
        <p>Site path: ${sitePath}</p>
      </body>
    </html>
  \`);
});

server.listen(port, hostname, () => {
  console.log(\`Server running at http://\${hostname}:\${port}/\`);
});`;
      await fs.writeFile(`${sitePath}/index.js`, content);
      break;
  }

  console.log(`✓ Created default files for ${type} site`);
}

/**
 * Ensure base directories exist
 */
export async function ensureBaseSiteDirectories(): Promise<void> {
  try {
    await fs.mkdir(SITES_BASE_DIR, { recursive: true, mode: 0o755 });
    console.log(`✓ Base site directory ready: ${SITES_BASE_DIR}`);
  } catch (error) {
    console.error('Error creating base site directories:', error);
    throw error;
  }
}
