import Component from '@ember/component';
import { inject as service } from '@ember/service';
import generateExecUrl from 'nomad-ui/utils/generate-exec-url';
import openExecUrl from 'nomad-ui/utils/open-exec-url';

export default Component.extend({
  tagName: '',

  router: service(),

  actions: {
    open() {
      openExecUrl(this.generateUrl());
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
