import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class Namespace extends ApplicationSerializer {
  primaryKey = 'Name';
}
