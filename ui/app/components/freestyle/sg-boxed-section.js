import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  variants: computed(() => [
    {
      key: 'Normal',
      title: 'Normal',
      slug: '',
    },
    {
      key: 'Info',
      title: 'Info',
      slug: 'is-info',
    },
    {
      key: 'Warning',
      title: 'Warning',
      slug: 'is-warning',
    },
    {
      key: 'Danger',
      title: 'Danger',
      slug: 'is-danger',
    },
  ]),
});
