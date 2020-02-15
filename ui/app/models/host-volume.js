import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  node: fragmentOwner(),

  name: attr('string'),
  path: attr('string'),
  readOnly: attr('boolean'),
});
