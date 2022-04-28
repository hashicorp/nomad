import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { alias } from '@ember/object/computed';
import classic from 'ember-classic-decorator';

@classic
export default class DefinitionController extends Controller.extend(WithNamespaceResetting) {
  @alias('model.job') job;
  @alias('model.definition') definition;

  isEditing = false;

  edit() {
    this.job.set('_newDefinition', JSON.stringify(this.definition, null, 2));
    this.set('isEditing', true);
  }

  onCancel() {
    this.set('isEditing', false);
  }

  onSubmit(id, jobNamespace) {
    this.transitionToRoute('jobs.job', id, {
      queryParams: { jobNamespace },
    });
  }
}
