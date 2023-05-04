import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class AuthMethodSerializer extends ApplicationSerializer {
  primaryKey = 'Name';
}
