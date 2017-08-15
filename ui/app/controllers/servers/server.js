import Ember from 'ember';

const { Controller } = Ember;

export default Controller.extend({
  activeTab: 'tags',

  actions: {
    setTab(tab) {
      this.set('activeTab', tab);
    },
  },
});
