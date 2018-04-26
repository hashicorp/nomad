import Component from '@ember/component';

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
  },
});
