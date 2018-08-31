import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { alias } from '@ember/object/computed';

export default Controller.extend(WithNamespaceResetting, {
  job: alias('model.job'),
  definition: alias('model.definition'),

  isEditing: false,

  edit() {
    this.get('job').set('_newDefinition', JSON.stringify(this.get('definition'), null, 2));
    this.set('isEditing', true);
  },

  onCancel() {
    this.set('isEditing', false);
  },

  onSubmit(id, namespace) {
    this.transitionToRoute('jobs.job', id, {
      queryParams: { jobNamespace: namespace },
    });
  },
});
