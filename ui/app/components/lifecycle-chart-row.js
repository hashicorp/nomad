import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  tagName: '',

  lifecycleString: computed('task.lifecycle', 'task.lifecycle.sidecar', function() {
    if (this.task.lifecycle) {
      if (this.task.lifecycle.sidecar) {
        return 'Sidecar';
      } else {
        return 'PreStart';
      }
    } else {
      return 'Main';
    }
  }),

  lifecycleClass: computed('lifecycleString', function() {
    return this.lifecycleString.toLowerCase();
  }),

  activeClass: computed('taskState.state', function() {
    if (this.taskState && this.taskState.state === 'running') {
      return 'is-active';
    }
  }),

  finishedClass: computed('taskState.finishedAt', function() {
    if (this.taskState && this.taskState.finishedAt) {
      return 'is-finished';
    }
  }),
});
