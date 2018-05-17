import Helper from '@ember/component/helper';

export function pluralize([term, count]) {
  return count === 1 ? term : term.pluralize();
}

export default Helper.helper(pluralize);
