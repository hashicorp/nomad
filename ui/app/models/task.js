import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentArray } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  name: attr('string'),
  driver: attr('string'),
  kind: attr('string'),

  reservedMemory: attr('number'),
  reservedCPU: attr('number'),
  reservedDisk: attr('number'),
  reservedEphemeralDisk: attr('number'),

  volumeMounts: fragmentArray('volume-mount', { defaultValue: () => [] }),
});
