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

  this.route('nodes', function() {
    this.route('node', { path: '/:node_id' });
    this.route('servers', function() {
      this.route('server', { path: '/:agent_id' });
    });
  });

  if (config.environment === 'development') {
    this.route('freestyle');
  }
});

export default Router;
