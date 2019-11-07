import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';

export default Fragment.extend({
  destinationName: attr('string'),
  localBindPort: attr('string'),
});
