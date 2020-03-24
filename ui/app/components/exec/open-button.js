import Component from '@ember/component';
import { inject as service } from '@ember/service';
import generateExecUrl from 'nomad-ui/utils/generate-exec-url';

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
    let urlSegments = {
      job: this.job.get('name'),
    };

    if (this.taskGroup) {
      urlSegments.taskGroup = this.taskGroup.get('name');
    }

    if (this.task) {
      urlSegments.task = this.task.get('name');
    }

    if (this.allocation) {
      urlSegments.allocation = this.allocation.get('shortId');
    }

    return generateExecUrl(this.router, urlSegments);
  },
});
