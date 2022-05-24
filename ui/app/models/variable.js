import Model from '@ember-data/model';
import { attr } from '@ember-data/model';
import classic from 'ember-classic-decorator';

@classic
export default class VariableModel extends Model {
  @attr('string') path;
  @attr('string') namespace;
  @attr() items;
}
