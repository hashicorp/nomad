import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragment } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  name: attr('string'),
  portLabel: attr('string'),
  tags: attr(),
  connect: fragment('consul-connect'),
});
