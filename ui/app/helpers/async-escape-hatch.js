import Helper from '@ember/component/helper';

export function asyncEscapeHatch([model, relationship]) {
  return model[relationship].content;
}

export default Helper.helper(asyncEscapeHatch);
