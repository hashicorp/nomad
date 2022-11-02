import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import notifyError from 'nomad-ui/utils/notify-error';
import { watchRecord } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import classic from 'ember-classic-decorator';

@classic
export default class VolumeRoute extends Route.extend(WithWatchers) {
  @service store;
  @service system;

  startWatchers(controller, model) {
    if (!model) return;

    controller.set('watchers', {
      model: this.watch.perform(model),
    });
  }

  serialize(model) {
    return { volume_name: JSON.parse(model.get('id')).join('@') };
  }

  async model(params) {
    // Issue with naming collissions
    const url = params.volume_name.split('@');
    const namespace = url.pop();
    const name = url.join('');

    const fullId = JSON.stringify([`csi/${name}`, namespace || 'default']);

    try {
      const [volume] = await Promise.all([
        this.store.findRecord('volume', fullId, {
          reload: true,
        }),
        this.store.findAll('namespace'),
      ]);

      return volume;
    } catch (e) {
      notifyError(this)(e);
    }
  }

  // Since volume includes embedded records for allocations,
  // it's possible that allocations that are server-side deleted may
  // not be removed from the UI while sitting on the volume detail page.
  @watchRecord('volume') watch;
  @collect('watch') watchers;
}
