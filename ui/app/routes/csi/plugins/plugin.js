import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import notifyError from 'nomad-ui/utils/notify-error';

export default class PluginRoute extends Route {
  @service store;
  @service system;

  breadcrumbs = plugin => [
    {
      label: 'Plugins',
      args: ['csi.plugins'],
    },
    {
      label: plugin.plainId,
      args: ['csi.plugins.plugin', plugin.plainId],
    },
  ];

  serialize(model) {
    return { plugin_name: model.get('plainId') };
  }

  model(params) {
    return this.store.findRecord('plugin', `csi/${params.plugin_name}`).catch(notifyError(this));
  }
}
