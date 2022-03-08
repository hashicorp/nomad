import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class Policy extends ApplicationSerializer {
  normalize(typeHash, hash) {
    hash.ID = hash.Name;
    return super.normalize(typeHash, hash);
  }
}
