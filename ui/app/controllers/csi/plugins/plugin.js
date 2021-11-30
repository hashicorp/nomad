import Controller from '@ember/controller';

export default class CsiPluginsPluginController extends Controller {
  get breadcrumbs() {
    const plugin = this.model;
    return [
      {
        label: 'Plugins',
        args: ['csi.plugins'],
      },
      {
        label: plugin.plainId,
        args: ['csi.plugins.plugin', plugin.plainId],
      },
    ];
  }
}
