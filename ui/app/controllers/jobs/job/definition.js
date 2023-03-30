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
  @alias('model.definition') definition;
  @alias('model.job') job;
  @alias('model.specification') specification;

  @tracked view;
  @tracked isEditing = false;
  queryParams = ['isEditing', 'view'];

  @service router;

  get context() {
    return this.isEditing ? 'edit' : 'read';
  }

  @action
  toggleEdit(bool) {
    this.isEditing = bool || !this.isEditing;
  }

  @action
  selectView(selectedView) {
    this.view = selectedView;
  }

  onSubmit() {
    this.router.transitionTo('jobs.job', this.job.idWithNamespace);
  }
}
