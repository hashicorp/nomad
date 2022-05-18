import Model from '@ember-data/model';
import { attr, belongsTo, hasMany } from '@ember-data/model';

export default class VarModel extends Model {
  @attr('string') path;
  @attr('string') key;
  @attr('string') value;
  @attr('string') namespace;
}
