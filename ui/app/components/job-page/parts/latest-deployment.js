import Component from '@ember/component';
import { task } from 'ember-concurrency';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

export default Component.extend({
  job: null,
  tagName: '',

  handleError() {},

  isShowingDeploymentDetails: false,

  promote: task(function*() {
    try {
      yield this.get('job.latestDeployment.content').promote();
    } catch (err) {
      let message = messageFromAdapterError(err);
      if (!message || message === 'Forbidden') {
        message = 'Your ACL token does not grant permission to promote deployments.';
      }
      this.get('handleError')({
        title: 'Could Not Promote Deployment',
        description: message,
      });
    }
  }),
});
