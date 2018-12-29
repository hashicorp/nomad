import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { alias } from '@ember/object/computed';

export default Controller.extend(WithNamespaceResetting, {
  job: alias('model'),
});
