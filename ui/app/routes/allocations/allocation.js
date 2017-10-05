import Ember from 'ember';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';

const { Route } = Ember;

export default Route.extend(WithModelErrorHandling);
