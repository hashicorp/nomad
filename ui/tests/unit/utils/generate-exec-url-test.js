import generateExecUrl from 'nomad-ui/utils/generate-exec-url';
import { module, test } from 'qunit';
import sinon from 'sinon';

const emptyOptions = { queryParams: {} };

module('Unit | Utility | generate-exec-url', function(hooks) {
  hooks.beforeEach(function() {
    this.urlForSpy = sinon.spy();
    this.router = { urlFor: this.urlForSpy, currentRoute: { queryParams: {} } };
  });

  test('it generates an exec job URL', function(assert) {
    generateExecUrl(this.router, { job: 'job-name' });

    assert.ok(this.urlForSpy.calledWith('exec', 'job-name', emptyOptions));
  });

  test('it generates an exec job URL with an allocation', function(assert) {
    generateExecUrl(this.router, { job: 'job-name', allocation: 'allocation-short-id' });

    assert.ok(
      this.urlForSpy.calledWith('exec', 'job-name', {
        queryParams: { allocation: 'allocation-short-id' },
      })
    );
  });

  test('it generates an exec task group URL', function(assert) {
    generateExecUrl(this.router, { job: 'job-name', taskGroup: 'task-group-name' });

    assert.ok(
      this.urlForSpy.calledWith('exec.task-group', 'job-name', 'task-group-name', emptyOptions)
    );
  });

  test('it generates an exec task URL', function(assert) {
    generateExecUrl(this.router, {
      allocation: 'allocation-short-id',
      job: 'job-name',
      taskGroup: 'task-group-name',
      task: 'task-name',
    });

    assert.ok(
      this.urlForSpy.calledWith(
        'exec.task-group.task',
        'job-name',
        'task-group-name',
        'task-name',
        { queryParams: { allocation: 'allocation-short-id' } }
      )
    );
  });

  test('it includes query parameters from the current route', function(assert) {
    this.router.currentRoute.queryParams = {
      namespace: 'a-namespace',
      region: 'a-region',
    };

    generateExecUrl(this.router, { job: 'job-name', allocation: 'id' });

    assert.ok(
      this.urlForSpy.calledWith('exec', 'job-name', {
        queryParams: { allocation: 'id', namespace: 'a-namespace', region: 'a-region' },
      })
    );
  });
});
