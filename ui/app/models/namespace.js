import { readOnly } from '@ember/object/computed';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export default class Namespace extends Model {
  @readOnly('id') name;
  @attr('string') hash;
  @attr('string') description;
}
