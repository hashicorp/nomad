import { helper } from '@ember/component/helper';

function createColumns(positional) {
  return positional.map((column) => ({ label: column }));
}

export default helper(createColumns);
