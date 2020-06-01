import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import notifyError from 'nomad-ui/utils/notify-error';

export default Route.extend({
  model({ path = '/' }) {
    const decodedPath = decodeURIComponent(path);
    const allocation = this.modelFor('allocations.allocation');

    if (!allocation.isRunning) {
      return {
        path: decodedPath,
        allocation,
      };
    }

    return RSVP.all([allocation.stat(decodedPath), allocation.get('node')])
      .then(([statJson]) => {
        if (statJson.IsDir) {
          return RSVP.hash({
            path: decodedPath,
            allocation,
            directoryEntries: allocation.ls(decodedPath).catch(notifyError(this)),
            isFile: false,
          });
        } else {
          return {
            path: decodedPath,
            allocation,
            isFile: true,
            stat: statJson,
          };
        }
      })
      .catch(notifyError(this));
  },

  setupController(controller, { path, allocation, directoryEntries, isFile, stat } = {}) {
    this._super(...arguments);
    controller.setProperties({ path, allocation, directoryEntries, isFile, stat });
  },
});
