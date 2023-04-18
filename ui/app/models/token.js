import { alias } from '@ember/object/computed';
import Model from '@ember-data/model';
import { attr } from '@ember-data/model';
import { hasMany } from '@ember-data/model';

export default class Token extends Model {
  @attr('string') secret;
  @attr('string') name;
  @attr('boolean') global;
  @attr('date') createTime;
  @attr('string') type;
  @hasMany('policy') policies;
  @attr() policyNames;

  @alias('id') accessor;
}
