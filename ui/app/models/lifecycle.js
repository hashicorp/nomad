import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  task: fragmentOwner(),

  hook: attr('string'),
  sidecar: attr('boolean'),
});
