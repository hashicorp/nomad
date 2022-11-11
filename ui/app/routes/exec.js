import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import notifyError from 'nomad-ui/utils/notify-error';
import { collect } from '@ember/object/computed';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import {
  watchRecord,
  watchRelationship,
} from 'nomad-ui/utils/properties/watch';
import classic from 'ember-classic-decorator';

@classic
export default class ExecRoute extends Route.extend(WithWatchers) {
  @service store;
  @service token;

  serialize(model) {
    return { job_name: model.get('plainId') };
  }

  async model(params, transition) {
    const namespace = transition.to.queryParams.namespace;
    const name = params.job_name;
    const fullId = JSON.stringify([name, namespace || 'default']);

    try {
      const [job, { Terminal }] = await Promise.all([
        this.store.findRecord('job', fullId),
        import('xterm'),
      ]);

      await job.get('allocations');

      return [job, Terminal];
    } catch (e) {
      notifyError.call(this, e);
    }
  }

  setupController(controller, [job, Terminal]) {
    super.setupController(controller, job);
    controller.setUpTerminal(Terminal);
  }

  startWatchers(controller, model) {
    if (model) {
      controller.set('watcher', this.watch.perform(model));
      controller.set('watchAllocations', this.watchAllocations.perform(model));
    }
  }

  @watchRecord('job') watch;
  @watchRelationship('allocations') watchAllocations;

  @collect('watch', 'watchAllocations') watchers;
}
