export function provide(count, provider) {
  if (typeof count === 'function') {
    count = count();
  }
  return Array(count).fill(null).map(provider);
}

export function provider() {
  return () => provide(...arguments);
}
