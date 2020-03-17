import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';

export default Fragment.extend({
  name: attr('string'),

  source: attr('string'),
  type: attr('string'),
  readOnly: attr('boolean'),
});
