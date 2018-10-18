import { lt, equal } from '@ember/object/computed';
import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';

export default Fragment.extend({
  deadline: attr('number'),
  forceDeadline: attr('date'),
  ignoreSystemJobs: attr('boolean'),

  isForced: lt('deadline', 0),
  hasNoDeadline: equal('deadline', 0),
});
