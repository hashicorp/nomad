import Model from '@ember-data/model';
import { attr } from '@ember-data/model';

export default class Policy extends Model {
  @attr('string') name;
  @attr('string') description;
  @attr('string') rules;
  @attr() rulesJSON;
}
