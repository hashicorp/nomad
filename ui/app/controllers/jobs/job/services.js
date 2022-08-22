import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { alias } from '@ember/object/computed';

export default class JobsJobServicesController extends Controller.extend(
  WithNamespaceResetting
) {
  @alias('model') job;
}
