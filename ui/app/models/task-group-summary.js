import Ember from 'ember';
import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

const { computed } = Ember;

export default Fragment.extend({
  job: fragmentOwner(),
  name: attr('string'),

  queuedAllocs: attr('number'),
  startingAllocs: attr('number'),
  runningAllocs: attr('number'),
  completeAllocs: attr('number'),
  failedAllocs: attr('number'),
  lostAllocs: attr('number'),

  allocsList: computed.collect(
    'queuedAllocs',
    'startingAllocs',
    'runningAllocs',
    'completeAllocs',
    'failedAllocs',
    'lostAllocs'
  ),

  totalAllocs: computed.sum('allocsList'),
});
