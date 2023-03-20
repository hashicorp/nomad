import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { alias } from '@ember/object/computed';
import classic from 'ember-classic-decorator';
import { inject as service } from '@ember/service';

@classic
export default class DefinitionController extends Controller.extend(
  WithNamespaceResetting
) {
  @alias('model.definition') definition;
  @alias('model.job') job;
  @alias('model.specification') specification;
  @alias('model.variables') variables;

  @service router;

  onSubmit() {
    this.router.transitionTo('jobs.job', this.job.idWithNamespace);
  }
}
