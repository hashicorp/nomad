import Controller from '@ember/controller';

export default Controller.extend({
  queryParams: {
    volumeNamespace: 'namespace',
  },

  isForbidden: false,

  volumeNamespace: 'default',
});
