import Watchable from './watchable';
import addToPath from 'nomad-ui/utils/add-to-path';
import classic from 'ember-classic-decorator';

@classic
export default class AllocationAdapter extends Watchable {
  stop = adapterAction('/stop');

  restart(allocation, taskName) {
    const prefix = `${this.host || '/'}${this.urlPrefix()}`;
    const url = `${prefix}/client/allocation/${allocation.id}/restart`;
    return this.ajax(url, 'PUT', {
      data: taskName && { TaskName: taskName },
    });
  }

  ls(model, path) {
    return this.token
      .authorizedRequest(
        `/v1/client/fs/ls/${model.id}?path=${encodeURIComponent(path)}`
      )
      .then(handleFSResponse);
  }

  stat(model, path) {
    return this.token
      .authorizedRequest(
        `/v1/client/fs/stat/${model.id}?path=${encodeURIComponent(path)}`
      )
      .then(handleFSResponse);
  }
}

async function handleFSResponse(response) {
  if (response.ok) {
    return response.json();
  } else {
    const body = await response.text();

    throw {
      code: response.status,
      toString: () => body,
    };
  }
}

function adapterAction(path, verb = 'POST') {
  return function (allocation) {
    const url = addToPath(
      this.urlForFindRecord(allocation.id, 'allocation'),
      path
    );
    return this.ajax(url, verb);
  };
}
