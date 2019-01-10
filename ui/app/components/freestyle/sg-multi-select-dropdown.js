import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  options: computed(() => [
    { key: 'option-1', label: 'Option One' },
    { key: 'option-2', label: 'Option Two' },
    { key: 'option-3', label: 'Option Three' },
    { key: 'option-4', label: 'Option Four' },
    { key: 'option-5', label: 'Option Five' },
  ]),

  selection: computed(() => ['option-2', 'option-4', 'option-5']),
});
