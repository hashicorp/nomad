import Component from '@ember/component';
import { computed, get } from '@ember/object';

export default Component.extend({
  classNames: ['accordion'],

  key: 'id',
  source: computed(() => []),

  onToggle(/* item, isOpen */) {},
  startExpanded: false,

  decoratedSource: computed('source.[]', function() {
    const stateCache = this.stateCache;
    const key = this.key;
    const deepKey = `item.${key}`;
    const startExpanded = this.startExpanded;

    const decoratedSource = this.source.map(item => {
      const cacheItem = stateCache.findBy(deepKey, get(item, key));
      return {
        item,
        isOpen: cacheItem ? !!cacheItem.isOpen : startExpanded,
      };
    });

    this.set('stateCache', decoratedSource);
    return decoratedSource;
  }),

  // When source updates come in, the state cache is used to preserve
  // open/close state.
  stateCache: computed(() => []),
});
