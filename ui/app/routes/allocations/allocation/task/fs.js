import Route from '@ember/routing/route';
import RSVP from 'rsvp';

export default Route.extend({
  model({ path = '/' }) {
    const decodedPath = decodeURIComponent(path);
    const task = this.modelFor('allocations.allocation.task');

    const pathWithTaskName = `${task.name}${decodedPath.startsWith('/') ? '' : '/'}${decodedPath}`;

    return task.stat(pathWithTaskName).then(statJson => {
      if (statJson.IsDir) {
        return RSVP.hash({
          path: decodedPath,
          task,
          directoryEntries: task.ls(pathWithTaskName),
          isFile: false,
        });
      } else {
        return {
          path: decodedPath,
          task,
          isFile: true,
        };
      }
    });
  },

  setupController(controller, { path, task, directoryEntries, isFile }) {
    this._super(...arguments);
    controller.setProperties({ path, model: task, directoryEntries, isFile });
  },
});
