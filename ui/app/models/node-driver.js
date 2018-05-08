import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';
import { fragment } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  node: fragmentOwner(),

  attributes: fragment('node-attributes'),
  name: attr('string'),
  detected: attr('boolean', { defaultValue: false }),
  healthy: attr('boolean', { defaultValue: false }),
  healthDescription: attr('string'),
  updateTime: attr('date'),
});
