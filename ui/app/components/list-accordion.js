import Component from '@ember/component';
import { computed, get } from '@ember/object';

export default Component.extend({
  classNames: ['accordion'],

  key: 'id',
  source: computed(() => []),

  decoratedSource: computed('source.[]', function() {
    const stateCache = this.get('stateCache');
    const key = this.get('key');
    const deepKey = `item.${key}`;

    const decoratedSource = this.get('source').map(item => {
      const cacheItem = stateCache.findBy(deepKey, get(item, key));
      return {
        item,
        isOpen: cacheItem ? !!cacheItem.isOpen : false,
      };
    });

    this.set('stateCache', decoratedSource);
    return decoratedSource;
  }),

  // When source updates come in, the state cache is used to preserve
  // open/close state.
  stateCache: computed(() => []),
});
