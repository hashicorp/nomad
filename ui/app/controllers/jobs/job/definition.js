import Controller from '@ember/controller';
import { action } from '@ember/object';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import classic from 'ember-classic-decorator';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';

@classic
export default class DefinitionController extends Controller.extend(
  WithNamespaceResetting
) {
  @tracked view = this.specification ? 'job-spec' : 'full-definition';
  queryParams = ['view'];

  @alias('model.definition') definition;
  @alias('model.job') job;
  @alias('model.specification') specification;

  @service router;

  @action
  toggleView() {
    const opposite = this.view === 'job-spec' ? 'full-definition' : 'job-spec';
    this.view = opposite;
  }

  onSubmit() {
    this.router.transitionTo('jobs.job', this.job.idWithNamespace);
  }
}
