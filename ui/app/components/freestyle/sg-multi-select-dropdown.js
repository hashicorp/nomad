import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  options1: computed(() => [
    { key: 'option-1', label: 'Option One' },
    { key: 'option-2', label: 'Option Two' },
    { key: 'option-3', label: 'Option Three' },
    { key: 'option-4', label: 'Option Four' },
    { key: 'option-5', label: 'Option Five' },
  ]),

  selection1: computed(() => ['option-2', 'option-4', 'option-5']),

  optionsMany: computed(() =>
    Array(100)
      .fill(null)
      .map((_, i) => ({ label: `Option ${i}`, key: `option-${i}` }))
  ),
  selectionMany: computed(() => []),

  optionsDatacenter: computed(() => [
    { key: 'pdx-1', label: 'pdx-1' },
    { key: 'jfk-1', label: 'jfk-1' },
    { key: 'jfk-2', label: 'jfk-2' },
    { key: 'muc-1', label: 'muc-1' },
  ]),
  selectionDatacenter: computed(() => ['jfk-1', 'jfk-2']),

  optionsType: computed(() => [
    { key: 'batch', label: 'Batch' },
    { key: 'service', label: 'Service' },
    { key: 'system', label: 'System' },
    { key: 'periodic', label: 'Periodic' },
    { key: 'parameterized', label: 'Parameterized' },
  ]),
  selectionType: computed(() => ['system', 'service']),

  optionsStatus: computed(() => [
    { key: 'pending', label: 'Pending' },
    { key: 'running', label: 'Running' },
    { key: 'dead', label: 'Dead' },
  ]),
  selectionStatus: computed(() => []),
});
