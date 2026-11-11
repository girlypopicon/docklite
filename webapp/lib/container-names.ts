/**
 * The name to show people for a container. Docker names like "docklite-site-example-com" are plumbing:
 * a site is shown as its domain, a database as its database name. Anything else keeps its Docker name.
 */
export function containerDisplayName(container: { name: string; labels?: { [key: string]: string } }): string {
  const labels = container.labels || {};
  if (labels['docklite.domain']) return labels['docklite.domain'];
  if (labels['docklite.database']) return labels['docklite.database'];
  return container.name.replace(/^docklite-(site|db)-/, '');
}
