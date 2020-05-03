import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import notifyError from 'nomad-ui/utils/notify-error';
import { watchRecord } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithWatchers, {
  store: service(),
  system: service(),

  breadcrumbs: plugin => [
    {
      label: 'Plugins',
      args: ['csi.plugins'],
    },
    {
      label: plugin.name,
      args: ['csi.plugins.plugin', plugin.name],
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

  model(params) {
    return this.store.findRecord('plugin', `csi/${params.plugin_name}`).catch(notifyError(this));
  },

  watch: watchRecord('plugin'),
  watchers: collect('watch'),
});
