import Component from '@ember/component';

export default Component.extend({
  tagName: '',

  actions: {
    open() {
      let job = this.job;
      // FIXME adapted from components#task-group-parent
      window.open(`/ui/exec/${job.name}`, '_blank', 'width=973,height=490,location=1');
    },
  },
});
