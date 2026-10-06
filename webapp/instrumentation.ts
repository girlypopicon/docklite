export async function register() {
  // The public demo never touches a database.
  if (process.env.NEXT_PUBLIC_DOCKLITE_DEMO === '1') return;
  if (process.env.NEXT_RUNTIME === 'nodejs') {
    // Initialize database and run migrations on server startup
    const { initializeDatabase } = await import('./lib/db');
    initializeDatabase();
    console.log('[DockLite] Database initialized and migrations completed');
  }
}
