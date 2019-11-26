import Component from '@ember/component';
import { computed } from '@ember/object';
import { computed as overridable } from 'ember-overridable-computed';

export default Component.extend({
  tagName: 'table',
  classNames: ['table'],

  source: overridable(() => []),

  // Plan for a future with metadata (e.g., isSelected)
  decoratedSource: computed('source.[]', function() {
    return this.source.map(row => ({
      model: row,
    }));
  }),
});
