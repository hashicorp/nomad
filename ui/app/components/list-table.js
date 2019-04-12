import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  tagName: 'table',
  classNames: ['table'],

  source: computed(() => []),

  // Plan for a future with metadata (e.g., isSelected)
  decoratedSource: computed('source.[]', function() {
    return this.source.map(row => ({
      model: row,
    }));
  }),
});
