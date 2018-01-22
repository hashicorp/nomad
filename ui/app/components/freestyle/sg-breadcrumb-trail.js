import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  breadcrumbData: computed(() => {
    return [
      {
        label: 'One',
        params: ['index'],
      },
      {
        label: 'Two',
        active: true,
        params: ['index'],
      },
    ];
  }),
});
