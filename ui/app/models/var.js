import Model from '@ember-data/model';
import { attr, belongsTo, hasMany } from '@ember-data/model';
import classic from 'ember-classic-decorator';

@classic
export default class VarModel extends Model {
  @attr('string') path;
  // @attr() key;
  // @attr('string') value;
  @attr('string') namespace;
  @attr() items;
}
