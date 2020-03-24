import generateExecUrl from 'nomad-ui/utils/generate-exec-url';
import { module, test } from 'qunit';
import sinon from 'sinon';

module('Unit | Utility | generate-exec-url', function(hooks) {
  hooks.beforeEach(function() {
    this.urlForSpy = sinon.spy();
    this.router = { urlFor: this.urlForSpy };
  });

  test('it generates an exec job URL', function(assert) {
    generateExecUrl(this.router, { job: 'job-name' });
    assert.ok(this.urlForSpy.calledWith('exec', 'job-name'));
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
    assert.ok(this.urlForSpy.calledWith('exec.task-group', 'job-name', 'task-group-name'));
  });

  test('it generates an exec task URL', function(assert) {
    generateExecUrl(this.router, {
      job: 'job-name',
      taskGroup: 'task-group-name',
      task: 'task-name',
    });
    assert.ok(
      this.urlForSpy.calledWith('exec.task-group.task', 'job-name', 'task-group-name', 'task-name')
    );
  });
});
