import Ember from 'ember';
import ApplicationSerializer from './application';

const { copy } = Ember;

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
