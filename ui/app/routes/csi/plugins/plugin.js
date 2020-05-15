import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import notifyError from 'nomad-ui/utils/notify-error';
import { watchRecord } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithWatchers, {
  store: service(),
  system: service(),

  breadcrumbs: model => [
    {
      label: 'Plugins',
      args: ['csi.plugins'],
    },
    {
      label: model.plugin.plainId,
      args: ['csi.plugins.plugin', model.plugin.plainId],
    },
  ],

  startWatchers(controller, model) {
    if (!model) return;

    controller.set('watchers', {
      model: this.watch.perform(model),
    });
  },

  serialize(model) {
    return { plugin_name: model.get('plainId') };
  },

  async model(params) {
    const plugin = await this.store
      .findRecord('plugin', `csi/${params.plugin_name}`)
      .catch(notifyError(this));

    const pluginController = plugin.get('controllers.firstObject');
    const pluginNode = plugin.get('nodes.firstObject');

    const controllerAllocation = pluginController && (await pluginController.getAllocation());
    const nodeAllocation = pluginNode && (await pluginNode.getAllocation());

    return {
      plugin,
      controllerAllocation,
      nodeAllocation,
    };
  },

  watch: watchRecord('plugin'),
  watchers: collect('watch'),
});
