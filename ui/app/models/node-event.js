import { alias } from '@ember/object/computed';
import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  node: fragmentOwner(),

  message: attr('string'),
  subsystem: attr('string'),
  details: attr(),
  time: attr('date'),

  driver: alias('details.driver'),
});
