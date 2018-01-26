import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  distributionBarData: computed(() => {
    return [
      { label: 'one', value: 10 },
      { label: 'two', value: 20 },
      { label: 'three', value: 0 },
      { label: 'four', value: 35 },
    ];
  }),
});
