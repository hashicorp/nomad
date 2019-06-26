import ApplicationAdapter from './application';
import { inject as service } from '@ember/service';

export default ApplicationAdapter.extend({
  token: service(),

  ls(model, path) {
    return this.token
      .authorizedRequest(`/v1/client/fs/ls/${model.allocation.id}?path=${path}`)
      .then(response => response.json());
  },

  stat(model, path) {
    return this.token
      .authorizedRequest(`/v1/client/fs/stat/${model.allocation.id}?path=${path}`)
      .then(response => response.json());
  },
});
