import Route from '@ember/routing/route';
import notifyError from 'nomad-ui/utils/notify-error';

export default class FsRoute extends Route {
  async model({ path = '/' }) {
    const decodedPath = decodeURIComponent(path);
    const taskState = this.modelFor('allocations.allocation.task');

    if (!taskState || !taskState.allocation) return;

    const allocation = taskState.allocation;
    const pathWithTaskName = `${taskState.name}${
      decodedPath.startsWith('/') ? '' : '/'
    }${decodedPath}`;

    try {
      const [statJson] = await Promise.all([
        allocation.stat(pathWithTaskName),
        taskState.get('allocation.node'),
      ]);

      if (statJson.IsDir) {
        const directoryEntries = await allocation.ls(pathWithTaskName);
        return {
          path: decodedPath,
          taskState,
          directoryEntries,
          isFile: false,
        };
      } else {
        return {
          path: decodedPath,
          taskState,
          isFile: true,
          stat: statJson,
        };
      }
    } catch (e) {
      notifyError.call(this, e);
    }

    return Promise.all([
      allocation.stat(pathWithTaskName),
      taskState.get('allocation.node'),
    ]);
  }

  setupController(
    controller,
    { path, taskState, directoryEntries, isFile, stat } = {}
  ) {
    super.setupController(...arguments);
    controller.setProperties({
      path,
      taskState,
      directoryEntries,
      isFile,
      stat,
    });
  }
}
