import faker from 'nomad-ui/mirage/faker';

export function provide(count, provider) {
  if (typeof count === 'function') {
    count = count();
  }
  return Array(count)
    .fill(null)
    .map(provider);
}

export function provider() {
  return () => provide(...arguments);
}

export function pickOne(list) {
  return list[faker.random.number(list.length - 1)];
}

export function arrToObj(prop, alias = '') {
  return (obj, element) => {
    const name = element[prop];
    delete element[prop];

    obj[name] = alias ? element[alias] : element;
    return obj;
  };
}
