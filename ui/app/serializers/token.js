import { copy } from '@ember/object/internals';
import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  primaryKey: 'AccessorID',

  attrs: {
    secret: 'SecretID',
  },

  normalize(typeHash, hash) {
    hash.PolicyIDs = hash.Policies;
    hash.PolicyNames = copy(hash.Policies);
    return this._super(typeHash, hash);
  },
});
