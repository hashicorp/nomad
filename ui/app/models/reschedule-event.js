import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';
import shortUUIDProperty from '../utils/properties/short-uuid';

export default Fragment.extend({
  allocation: fragmentOwner(),

  previousAllocationId: attr('string'),
  previousNodeId: attr('string'),
  time: attr('date'),
  delay: attr('string'),

  previousAllocationShortId: shortUUIDProperty('previousAllocationId'),
  previousNodeShortId: shortUUIDProperty('previousNodeShortId'),
});
