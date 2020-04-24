import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragment, fragmentArray, fragmentOwner } from 'ember-data-model-fragments/attributes';
import { computed } from '@ember/object';

export default Fragment.extend({
  taskGroup: fragmentOwner(),

  name: attr('string'),
  driver: attr('string'),
  kind: attr('string'),

  lifecycle: fragment('lifecycle'),

  lifecycleName: computed('lifecycle', 'lifecycle.sidecar', function() {
    if (this.lifecycle) {
      if (this.lifecycle.sidecar) {
        return 'sidecar';
      } else {
        return 'prestart';
      }
    } else {
      return 'main';
    }
  }),

  reservedMemory: attr('number'),
  reservedCPU: attr('number'),
  reservedDisk: attr('number'),
  reservedEphemeralDisk: attr('number'),

  volumeMounts: fragmentArray('volume-mount', { defaultValue: () => [] }),
});
