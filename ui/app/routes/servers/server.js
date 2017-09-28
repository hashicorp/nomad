import Ember from 'ember';
import notifyError from 'nomad-ui/utils/notify-error';

const { Route } = Ember;

export default Route.extend({
  model() {
    return this._super(...arguments).catch(notifyError(this));
  },
});
