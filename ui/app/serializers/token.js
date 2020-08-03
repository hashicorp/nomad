import { copy } from 'ember-copy';
import ApplicationSerializer from './application';

export default class TokenSerializer extends ApplicationSerializer {
  primaryKey = 'AccessorID';

  attrs = {
    secret: 'SecretID',
  };

  normalize(typeHash, hash) {
    hash.PolicyIDs = hash.Policies;
    hash.PolicyNames = copy(hash.Policies);
    return super.normalize(typeHash, hash);
  }
}
