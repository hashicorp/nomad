import { readOnly } from '@ember/object/computed';
import Model from '@ember-data/model';
import { attr } from '@ember-data/model';

export default class Namespace extends Model {
  @readOnly('id') name;
  @attr('string') hash;
  @attr('string') description;
}
