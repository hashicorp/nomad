import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentArray } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  cpu: attr('number'),
  memory: attr('number'),
  disk: attr('number'),
  iops: attr('number'),
  networks: fragmentArray('network', { defaultValue: () => [] }),
});
