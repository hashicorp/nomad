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
    return generateExecUrl(this.router, {
      job: this.job,
      taskGroup: this.taskGroup,
      task: this.task,
      allocation: this.task
    });
  },
});
