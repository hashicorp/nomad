import Ember from 'ember';
import config from './config/environment';

const Router = Ember.Router.extend({
  location: config.locationType,
  rootURL: config.rootURL,
});

Router.map(function() {
  this.route('jobs', function() {
    this.route('job', { path: '/:job_id' });
  });

  // DEV BUILDS ONLY: Styleguide
  this.route('freestyle');
});

export default Router;
