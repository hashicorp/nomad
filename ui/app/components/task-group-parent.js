import Component from '@ember/component';

export default Component.extend({
  isOpen: false,

  actions: {
    toggleOpen() {
      this.toggleProperty('isOpen');
    },
  },
});
