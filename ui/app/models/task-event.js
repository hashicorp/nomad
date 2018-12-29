import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  state: fragmentOwner(),

  type: attr('string'),
  signal: attr('number'),
  exitCode: attr('number'),

  time: attr('date'),
  timeNanos: attr('number'),

  message: attr('string'),
});
