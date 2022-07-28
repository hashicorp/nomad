import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class PolicySerializer extends ApplicationSerializer {
  primaryKey = 'Name';

  normalize(typeHash, hash) {
    hash.ID = hash.Name;
    return super.normalize(typeHash, hash);
  }
}
