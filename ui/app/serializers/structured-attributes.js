import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class StructuredAttributes extends ApplicationSerializer {
  normalize(typeHash, hash) {
    return super.normalize(typeHash, { Raw: hash });
  }
}
