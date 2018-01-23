import Controller from '@ember/controller';
import { computed } from '@ember/object';

export default Controller.extend({
  activeTab: 'tags',

  sortedTags: computed('model.tags', function() {
    const tags = this.get('model.tags') || {};
    return Object.keys(tags)
      .map(name => ({
        name,
        value: tags[name],
      }))
      .sortBy('name');
  }),

  actions: {
    setTab(tab) {
      this.set('activeTab', tab);
    },
  },
});
