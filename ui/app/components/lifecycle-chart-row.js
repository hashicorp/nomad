import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  tagName: '',

  activeClass: computed('taskState.state', function() {
    if (this.taskState && this.taskState.state === 'running') {
      return 'is-active';
    }

    return;
  }),

  finishedClass: computed('taskState.finishedAt', function() {
    if (this.taskState && this.taskState.finishedAt) {
      return 'is-finished';
    }

    return;
  }),
});
