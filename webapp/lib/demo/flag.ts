// Public demo mode (demo.docklite.net). When the dashboard is built with
// NEXT_PUBLIC_DOCKLITE_DEMO=1, nothing talks to a real server: every /api call and the
// terminal are answered inside the visitor's browser from fake data, and that data
// disappears when the tab closes. See lib/demo/install.ts.
export const DEMO = process.env.NEXT_PUBLIC_DOCKLITE_DEMO === '1';
