import Route from '@ember/routing/route';
import fetch from 'nomad-ui/utils/fetch';
import RSVP from 'rsvp';

export default Route.extend({
  model({ path }) {
    const decodedPath = decodeURIComponent(path);
    const task = this.modelFor('allocations.allocation.task');

    const pathWithTaskName = `${task.name}${decodedPath}`;

    const allocation = this.modelFor('allocations.allocation');

    return fetch(`/v1/client/fs/stat/${allocation.id}?path=${pathWithTaskName}`)
      .then(response => response.json())
      .then(statJson => {
        if (statJson.IsDir) {
          return RSVP.hash({
            path: decodedPath,
            pathWithTaskName,
            task,
            ls: fetch(`/v1/client/fs/ls/${allocation.id}?path=${pathWithTaskName}`).then(response =>
              response.json()
            ),
            isFile: false,
          });
        } else {
          return {
            path: decodedPath,
            pathWithTaskName,
            task,
            isFile: true,
          };
        }
      });
  },

  setupController(controller, { path, pathWithTaskName, task, ls, isFile }) {
    this._super(...arguments);
    controller.setProperties({ path, pathWithTaskName, model: task, ls, isFile });
  },
});
