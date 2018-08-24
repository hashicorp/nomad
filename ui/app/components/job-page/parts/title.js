import Component from '@ember/component';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

export default Component.extend({
  tagName: '',

  job: null,
  title: null,

  handleError() {},

  actions: {
    stopJob() {
      this.get('job')
        .stop()
        .catch(() => {
          this.get('handleError')({
            title: 'Could Not Stop Job',
            description: 'Your ACL token does not grant permission to stop jobs.',
          });
        });
    },

    startJob() {
      const job = this.get('job');
      job
        .fetchRawDefinition()
        .then(definition => {
          // A stopped job will have this Stop = true metadata
          // If Stop is true when submitted to the cluster, the job
          // won't transition from the Dead to Running state.
          delete definition.Stop;
          job.set('_newDefinition', JSON.stringify(definition));
        })
        .then(() => {
          return job.parse();
        })
        .then(() => {
          return job.update();
        })
        .catch(err => {
          let message = messageFromAdapterError(err);
          if (!message || message === 'Forbidden') {
            message = 'Your ACL token does not grant permission to stop jobs.';
          }

          this.get('handleError')({
            title: 'Could Not Start Job',
            description: message,
          });
        });
    },
  },
});
