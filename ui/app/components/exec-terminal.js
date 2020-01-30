import Component from '@ember/component';

export default Component.extend({
  didInsertElement() {
    this.terminal.open(this.element.querySelector('.terminal'));
  },
});
