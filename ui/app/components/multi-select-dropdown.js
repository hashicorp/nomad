import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  classNames: ['dropdown'],

  options: computed(() => []),
  selection: computed(() => []),

  onSelect() {},

  actions: {
    toggle({ key }) {
      const newSelection = this.get('selection').slice();
      if (newSelection.includes(key)) {
        newSelection.removeObject(key);
      } else {
        newSelection.addObject(key);
      }
      this.get('onSelect')(newSelection);
    },
  },
});
