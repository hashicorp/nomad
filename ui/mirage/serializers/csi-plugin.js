import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  embed: true,
  include: ['nodes', 'controllers', 'controllerJobs', 'nodeJobs'],
  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    if (json instanceof Array) {
      json.forEach(serializePlugin);
    } else {
      serializePlugin(json);
    }
    return json;
  },
});

function serializePlugin(plugin) {
  plugin.ControllerJobs = plugin.ControllerJobs.map(({ ID, Namespace, Version }) => ({
    ID,
    Namespace,
    Version,
  }));
  plugin.NodeJobs = plugin.NodeJobs.map(({ ID, Namespace, Version }) => ({
    ID,
    Namespace,
    Version,
  }));

  if (!plugin.ControllerJobs.length) plugin.ControllerJobs = null;
}
