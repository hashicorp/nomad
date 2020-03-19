import Component from '@ember/component';
import { inject as service } from '@ember/service';

export default Component.extend({
  tagName: '',

  router: service(),

  actions: {
    open() {
      // FIXME adapted from components#task-group-parent
      window.open(this.generateUrl(), '_blank', 'width=973,height=490,location=1');
    },
  },

  generateUrl() {
    if (this.taskGroup) {
      return this.router.urlFor(
        'exec.task-group',
        this.job.get('name'),
        this.taskGroup.get('name')
      );
    } else if (this.allocation) {
      let urlOptions = {
        queryParams: {
          allocation: this.allocation.shortId,
        },
      };

      return this.router.urlFor('exec', this.job.get('name'), urlOptions);
    } else {
      return this.router.urlFor('exec', this.job.get('name'));
    }
  },
});
