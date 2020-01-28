import Component from '@ember/component';

export default Component.extend({
  isOpen: true,

  actions: {
    toggleOpen() {
      this.toggleProperty('isOpen');
    },
  },
});
