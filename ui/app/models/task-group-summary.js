import { collect, sum } from '@ember/object/computed';
import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  job: fragmentOwner(),
  name: attr('string'),

  queuedAllocs: attr('number'),
  startingAllocs: attr('number'),
  runningAllocs: attr('number'),
  completeAllocs: attr('number'),
  failedAllocs: attr('number'),
  lostAllocs: attr('number'),

  allocsList: collect(
    'queuedAllocs',
    'startingAllocs',
    'runningAllocs',
    'completeAllocs',
    'failedAllocs',
    'lostAllocs'
  ),

  totalAllocs: sum('allocsList'),
});
