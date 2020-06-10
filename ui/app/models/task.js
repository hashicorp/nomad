import attr from 'ember-data/attr';
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
    if (this.lifecycle && this.lifecycle.sidecar) return 'sidecar';
    if (this.lifecycle && this.lifecycle.hook === 'prestart') return 'prestart';
    return 'main';
  }

  @attr('number') reservedMemory;
  @attr('number') reservedCPU;
  @attr('number') reservedDisk;
  @attr('number') reservedEphemeralDisk;

  @fragmentArray('volume-mount', { defaultValue: () => [] }) volumeMounts;
}
