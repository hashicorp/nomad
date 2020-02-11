import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';

export default Fragment.extend({
  volume: attr('string'),

  destination: attr('string'),
  propagationMode: attr('string'),
  readOnly: attr('boolean'),
});
