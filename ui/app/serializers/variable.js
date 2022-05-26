import classic from 'ember-classic-decorator';
import ApplicationSerializer from './application';

@classic
export default class VariableSerializer extends ApplicationSerializer {
  primaryKey = 'Path';
}
