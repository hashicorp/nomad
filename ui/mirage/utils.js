import { faker } from 'ember-cli-mirage';

export function provide(count, provider) {
  if (typeof count === 'function') {
    count = count();
  }
  return Array(count).fill(null).map(provider);
}

export function provider() {
  return () => provide(...arguments);
}

export function pickOne(list) {
  return list[faker.random.number(list.length - 1)];
}
