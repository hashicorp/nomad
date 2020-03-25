import EmberRouter from '@ember/routing/router';
import config from './config/environment';

const Router = EmberRouter.extend({
  location: config.locationType,
  rootURL: config.rootURL,
});

Router.map(function() {
  this.route('exec', { path: '/exec/:job_name' }, function() {
    this.route('task-group', { path: '/:task_group_name' }, function() {
      this.route('task', { path: '/:task_name' });
    });
  });

  this.route('jobs', function() {
    this.route('run');
    this.route('job', { path: '/:job_name' }, function() {
      this.route('task-group', { path: '/:name' });
      this.route('definition');
      this.route('versions');
      this.route('deployments');
      this.route('evaluations');
      this.route('allocations');
    });
  });

  this.route('clients', function() {
    this.route('client', { path: '/:node_id' });
  });

  this.route('servers', function() {
    this.route('server', { path: '/:agent_id' });
  });

  this.route('csi', function() {
    this.route('volumes', function() {
      this.route('volume', { path: '/:volume_name' });
    });
  });

  this.route('allocations', function() {
    this.route('allocation', { path: '/:allocation_id' }, function() {
      this.route('task', { path: '/:name' }, function() {
        this.route('logs');
        this.route('fs-root', { path: '/fs' });
        this.route('fs', { path: '/fs/*path' });
      });
    });
  });

  this.route('settings', function() {
    this.route('tokens');
  });

  this.route('not-found', { path: '/*' });
});

export default Router;
