import Component from '@ember/component';
import { task } from 'ember-concurrency';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

export default Component.extend({
  tagName: '',

  job: null,
  title: null,

  handleError() {},

  stopJob: task(function*() {
    try {
      const job = this.get('job');
      yield job.stop();
      // Eagerly update the job status to avoid flickering
      this.job.set('status', 'dead');
    } catch (err) {
      this.get('handleError')({
        title: 'Could Not Stop Job',
        description: 'Your ACL token does not grant permission to stop jobs.',
      });
    }
  }),

  startJob: task(function*() {
    const job = this.get('job');
    const definition = yield job.fetchRawDefinition();

    delete definition.Stop;
    job.set('_newDefinition', JSON.stringify(definition));

    try {
      yield job.parse();
      yield job.update();
      // Eagerly update the job status to avoid flickering
      job.set('status', 'running');
    } catch (err) {
      let message = messageFromAdapterError(err);
      if (!message || message === 'Forbidden') {
        message = 'Your ACL token does not grant permission to stop jobs.';
      }

      this.get('handleError')({
        title: 'Could Not Start Job',
        description: message,
      });
    }
  }),
});
