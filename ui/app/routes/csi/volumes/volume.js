import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import notifyError from 'nomad-ui/utils/notify-error';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';

export default Route.extend({
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

  serialize(model) {
    return { volume_name: model.get('plainId') };
  },

  model(params, transition) {
    const namespace = transition.to.queryParams.namespace || this.get('system.activeNamespace.id');
    const name = params.volume_name;
    const fullId = JSON.stringify([`csi/${name}`, namespace || 'default']);
    return this.store.findRecord('volume', fullId, { reload: true }).catch(notifyError(this));
  },
});
