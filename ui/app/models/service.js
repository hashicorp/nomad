import { attr } from '@ember-data/model';
// import { fragment } from 'ember-data-model-fragments/attributes';
import Model from '@ember-data/model';
import { alias } from '@ember/object/computed';

export default class Service extends Model {
  @attr('string') ServiceName;
  @alias('ServiceName') name;
  @attr() tags;
}
