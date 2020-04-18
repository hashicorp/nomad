import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import notifyError from 'nomad-ui/utils/notify-error';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';
import { watchRecord } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithWatchers, {
  store: service(),
  system: service(),

  breadcrumbs: volume => [
    {
      label: 'Volumes',
      args: [
        'csi.volumes',
        qpBuilder({ volumeNamespace: volume.get('namespace.name') || 'default' }),
      ],
    },
    {
      label: volume.name,
      args: [
        'csi.volumes.volume',
        volume.plainId,
        qpBuilder({ volumeNamespace: volume.get('namespace.name') || 'default' }),
      ],
    },
  ],

  startWatchers(controller, model) {
    if (!model) return;

    controller.set('watchers', {
      model: this.watch.perform(model),
    });
  },

  serialize(model) {
    return { volume_name: model.get('plainId') };
  },

  model(params, transition) {
    const namespace = transition.to.queryParams.namespace || this.get('system.activeNamespace.id');
    const name = params.volume_name;
    const fullId = JSON.stringify([`csi/${name}`, namespace || 'default']);
    return this.store.findRecord('volume', fullId, { reload: true }).catch(notifyError(this));
  },

  // Since volume includes embedded records for allocations,
  // it's possible that allocations that are server-side deleted may
  // not be removed from the UI while sitting on the volume detail page.
  watch: watchRecord('volume'),
  watchers: collect('watch'),
});
