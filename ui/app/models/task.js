import { attr } from '@ember-data/model';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragment, fragmentArray, fragmentOwner } from 'ember-data-model-fragments/attributes';
import { computed } from '@ember/object';

export default class Task extends Fragment {
  @fragmentOwner() taskGroup;

  @attr('string') name;
  @attr('string') driver;
  @attr('string') kind;

  @fragment('lifecycle') lifecycle;

  @computed('lifecycle', 'lifecycle.sidecar')
  get lifecycleName() {
    if (this.lifecycle) {
      const { hook, sidecar } = this.lifecycle;

      if (hook === 'prestart') {
        return sidecar ? 'prestart-sidecar' : 'prestart-ephemeral';
      } else if (hook === 'poststart') {
        return sidecar ? 'poststart-sidecar' : 'poststart-ephemeral';
      } else if (hook === 'poststop') {
        return 'poststop';
      }
    }

    return 'main';
  }

  @attr('number') reservedMemory;
  @attr('number') reservedMemoryMax;
  @attr('number') reservedCPU;
  @attr('number') reservedDisk;
  @attr('number') reservedEphemeralDisk;

  @fragmentArray('volume-mount', { defaultValue: () => [] }) volumeMounts;
}
