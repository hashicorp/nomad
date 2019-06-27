import ApplicationAdapter from './application';
import { inject as service } from '@ember/service';

export default ApplicationAdapter.extend({
  token: service(),

  ls(model, path) {
    return this.token
      .authorizedRequest(`/v1/client/fs/ls/${model.allocation.id}?path=${path}`)
      .then(handleFSResponse);
  },

  stat(model, path) {
    return this.token
      .authorizedRequest(`/v1/client/fs/stat/${model.allocation.id}?path=${path}`)
      .then(handleFSResponse);
  },
});

async function handleFSResponse(response) {
  if (response.ok) {
    return response.json();
  } else {
    const body = await response.text();

    // FIXME is this the same across all platforms?
    const statusIs500 = response.status === 500;
    const bodyIncludes404Text = body.includes('no such file or directory');

    const translatedCode = statusIs500 && bodyIncludes404Text ? 404 : response.status;

    throw {
      code: translatedCode,
      toString: () => body,
    };
  }
}
