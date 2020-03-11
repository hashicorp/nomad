import Component from '@ember/component';
import { inject as service } from '@ember/service';

export default Component.extend({
  tagName: '',

  router: service(),

  actions: {
    open() {
      let job = this.job;
      let url = this.router.urlFor('exec', job.name);
      // FIXME adapted from components#task-group-parent
      window.open(url, '_blank', 'width=973,height=490,location=1');
    },
  },
});
