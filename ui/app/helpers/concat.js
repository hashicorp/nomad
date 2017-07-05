import Ember from 'ember';

const { Helper } = Ember;

export function concatHelper(params) {
  return params.join('');
}

export default Helper.helper(concatHelper);
