import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { alias } from '@ember/object/computed';
import classic from 'ember-classic-decorator';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';

@classic
export default class DefinitionController extends Controller.extend(
  WithNamespaceResetting
) {
  @tracked view = 'job-spec';
  queryParams = ['view'];

  @alias('model.definition') definition;
  @alias('model.job') job;
  @alias('model.specification') specification;

  @service router;

  onSubmit() {
    this.router.transitionTo('jobs.job', this.job.idWithNamespace);
  }
}
