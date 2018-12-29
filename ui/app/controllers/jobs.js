import { inject as service } from '@ember/service';
import Controller from '@ember/controller';

export default Controller.extend({
  system: service(),

  queryParams: {
    jobNamespace: 'namespace',
  },

  isForbidden: false,

  jobNamespace: 'default',
});
