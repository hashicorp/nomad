import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { alias } from '@ember/object/computed';
import classic from 'ember-classic-decorator';
import { inject as service } from '@ember/service';

@classic
export default class DefinitionController extends Controller.extend(
  WithNamespaceResetting
) {
  @alias('model.job') job;
  @alias('model.definition') definition;
  @service router;

  isEditing = false;

  edit() {
    this.job.set('_newDefinition', JSON.stringify(this.definition, null, 2));
    this.set('isEditing', true);
  }

  onCancel() {
    this.set('isEditing', false);
  }

  onSubmit() {
    this.router.transitionTo('jobs.job', this.job.idWithNamespace);
  }
}
