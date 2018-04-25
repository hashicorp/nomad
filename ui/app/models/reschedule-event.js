import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  allocation: fragmentOwner(),

  previousAllocationID: attr('string'),
  previousNodeID: attr('string'),
  time: attr('date'),
  delay: attr('string'),
});
