import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';

export default Fragment.extend({
  name: attr('string'),
  driver: attr('string'),

  reservedMemory: attr('number'),
  reservedCPU: attr('number'),
  reservedDisk: attr('number'),
  reservedEphemeralDisk: attr('number'),
});
